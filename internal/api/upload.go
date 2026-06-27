package api

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

// smallFileLimit is the threshold below which a single multipart upload is used.
const smallFileLimit = 10 * 1024 * 1024 // 10 MB

// chunkSize is the size of each part for chunked uploads.
const chunkSize = 4 * 1024 * 1024 // 4 MB

// UploadResult is the file object returned by the API after a successful upload.
type UploadResult struct {
	ObjectKey  string `json:"objectKey"`
	ObjectName string `json:"objectName"`
	SizeInByte int64  `json:"sizeInBytes"`
	URL        string `json:"url"`
}

// Upload uploads a local file into the given folder, choosing the single-shot or
// chunked strategy based on file size. folderKey may be empty for the root.
func (c *Client) Upload(path, folderKey string) (*UploadResult, error) {
	if folderKey == "" {
		user, err := c.UserInfo()
		if err != nil {
			return nil, fmt.Errorf("getting root folder key: %w", err)
		}
		folderKey = user.RootFolderKey
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	if info.IsDir() {
		return nil, fmt.Errorf("%s is a directory", path)
	}

	if info.Size() < smallFileLimit {
		return c.uploadSmall(path, info.Size(), folderKey)
	}
	return c.uploadChunked(path, info.Size(), folderKey)
}

func md5File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *Client) uploadSmall(path string, size int64, folderKey string) (*UploadResult, error) {
	hash, err := md5File(path)
	if err != nil {
		return nil, err
	}

	name := filepath.Base(path)
	metadata := map[string]any{
		"name":      name,
		"size":      size,
		"hash":      hash,
		"folderKey": folderKey,
	}
	metaJSON, err := json.Marshal(metadata)
	if err != nil {
		return nil, err
	}

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	if err := w.WriteField("metadataString", string(metaJSON)); err != nil {
		return nil, err
	}
	fw, err := w.CreateFormFile("file", name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	if _, err := io.Copy(fw, f); err != nil {
		return nil, err
	}
	if err := w.Close(); err != nil {
		return nil, err
	}

	req, err := http.NewRequest(http.MethodPost, uploadHost+"/upload/files", &body)
	if err != nil {
		return nil, err
	}
	c.setCommonHeaders(req.Header)
	// multipart writer generates the Content-Type with the correct boundary.
	req.Header.Set("Content-Type", w.FormDataContentType())

	return c.doUpload(req, metaJSON)
}

type initiateResp struct {
	TransactionID string `json:"transactionId"`
	Offset        int64  `json:"offset"`
}

func (c *Client) uploadChunked(path string, size int64, folderKey string) (*UploadResult, error) {
	hash, err := md5File(path)
	if err != nil {
		return nil, err
	}
	name := filepath.Base(path)

	// 1. initiate
	initBody, err := json.Marshal(map[string]any{
		"name":      name,
		"size":      size,
		"hash":      hash,
		"folderKey": folderKey,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodPost, uploadHost+"/upload/files/chunked/initiate", bytes.NewReader(initBody))
	if err != nil {
		return nil, err
	}
	c.setCommonHeaders(req.Header)
	req.Header.Set("Content-Type", "application/json")

	respBody, status, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, fmt.Errorf("initiate failed: status %d: %s (request body: %s)", status, respBody, string(initBody))
	}

	// Detect instant upload (deduplication): the server returns the file object.
	var result UploadResult
	if err := json.Unmarshal(respBody, &result); err == nil && result.ObjectKey != "" {
		return &result, nil
	}

	var init initiateResp
	if err := json.Unmarshal(respBody, &init); err != nil {
		return nil, fmt.Errorf("parsing initiate response: %w", err)
	}

	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// 2. upload chunks, driven by the offset returned by the server (resumable).
	offset := init.Offset
	buf := make([]byte, chunkSize)
	for offset < size {
		if _, err := f.Seek(offset, io.SeekStart); err != nil {
			return nil, err
		}
		n, err := io.ReadFull(f, buf)
		if err != nil && err != io.ErrUnexpectedEOF && err != io.EOF {
			return nil, err
		}
		if n == 0 {
			break
		}
		chunk := buf[:n]

		sum := md5.Sum(chunk)
		chunkMD5 := hex.EncodeToString(sum[:])

		url := fmt.Sprintf("%s/upload/files/chunked?uploadId=%s", uploadHost, init.TransactionID)
		creq, err := http.NewRequest(http.MethodPut, url, bytes.NewReader(chunk))
		if err != nil {
			return nil, err
		}
		c.setCommonHeaders(creq.Header)
		creq.Header.Set("Content-Type", "application/octet-stream")
		creq.Header.Set("Content-MD5", chunkMD5)
		creq.Header.Set("X-Offset", fmt.Sprintf("%d", offset))

		cbody, cstatus, err := c.do(creq)
		if err != nil {
			return nil, err
		}
		if cstatus != http.StatusOK && cstatus != http.StatusCreated {
			return nil, fmt.Errorf("chunk at offset %d failed: status %d: %s", offset, cstatus, cbody)
		}

		// The final chunk response is the file object; intermediate ones carry
		// the next offset. Detect completion by the presence of objectKey.
		var next initiateResp
		if err := json.Unmarshal(cbody, &next); err == nil && next.Offset > offset {
			offset = next.Offset
			fmt.Printf("\ruploaded %d / %d bytes", offset, size)
			continue
		}

		// No advancing offset -> treat as the terminal response.
		var result UploadResult
		if err := json.Unmarshal(cbody, &result); err == nil && result.ObjectKey != "" {
			fmt.Printf("\ruploaded %d / %d bytes\n", size, size)
			return &result, nil
		}
		// Otherwise advance past this chunk and keep going.
		offset += int64(n)
	}

	return nil, fmt.Errorf("upload finished without a final file object response")
}

func (c *Client) doUpload(req *http.Request, metadata []byte) (*UploadResult, error) {
	body, status, err := c.do(req)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK && status != http.StatusCreated {
		return nil, fmt.Errorf("upload failed: status %d: %s (metadata: %s)", status, body, string(metadata))
	}
	var result UploadResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parsing upload response: %w", err)
	}
	return &result, nil
}

// do executes a request and returns the body and status code.
func (c *Client) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, err
	}
	return body, resp.StatusCode, nil
}
