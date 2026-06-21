package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

// Trash moves a file or folder to the trash.
func (c *Client) Trash(obj Object) error {
	type trashObject struct {
		ParentObjectKey  string `json:"parentObjectKey"`
		ParentObjectType string `json:"parentObjectType"`
		ObjectKey        string `json:"objectKey"`
		ObjectType       string `json:"objectType"`
		ObjectName       string `json:"objectName"`
		Status           string `json:"status"`
	}
	type trashReq struct {
		CorrelationId string      `json:"correlationId"`
		Object        trashObject `json:"object"`
		Operation     string      `json:"operation"`
	}

	payload := map[string]interface{}{
		"objects": []trashReq{
			{
				CorrelationId: c.creds.UserID,
				Object: trashObject{
					ParentObjectKey:  obj.ParentObjectKey,
					ParentObjectType: obj.ParentObjectType,
					ObjectKey:        obj.ObjectKey,
					ObjectType:       obj.ObjectType,
					ObjectName:       obj.ObjectName,
					Status:           "T",
				},
				Operation: "TRASH",
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest(http.MethodPut, apiHost+"/nms/metadata/1.0", bytes.NewReader(body))
	if err != nil {
		return err
	}
	c.setCommonHeaders(req.Header)
	req.Header.Set("Content-Type", "application/json; charset=UTF-8")

	respBody, status, err := c.do(req)
	if err != nil {
		return err
	}
	if status != http.StatusOK {
		return fmt.Errorf("trash %s failed: status %d: %s", obj.ObjectKey, status, respBody)
	}

	return nil
}
