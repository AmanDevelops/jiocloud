package api

import (
	"fmt"
	"strings"
)

// ResolvePath walks a slash-separated remote path starting from the user's root folder,
// and returns the matching Object. It returns an error if any segment is not found.
// An empty path or "/" returns a virtual object representing the root folder.
func (c *Client) ResolvePath(remotePath string) (Object, error) {
	user, err := c.UserInfo()
	if err != nil {
		return Object{}, err
	}

	rootObj := Object{
		ObjectKey:  user.RootFolderKey,
		ObjectType: TypeFolder,
		ObjectName: "",
	}

	segments := splitPath(remotePath)
	if len(segments) == 0 {
		return rootObj, nil
	}

	current := rootObj
	for i, seg := range segments {
		if current.ObjectType != TypeFolder {
			return Object{}, fmt.Errorf("segment %q is not a folder", segments[i-1])
		}

		objs, err := c.ListFolder(current.ObjectKey)
		if err != nil {
			return Object{}, fmt.Errorf("listing %s: %w", current.ObjectName, err)
		}

		found := false
		for _, o := range objs {
			if o.ObjectName == seg {
				current = o
				found = true
				break
			}
		}

		if !found {
			return Object{}, fmt.Errorf("path %q not found (failed at %q)", remotePath, seg)
		}
	}

	return current, nil
}

// MkdirAll creates a remote path of folders, creating any missing intermediate segments.
// Returns the final folder's Object.
func (c *Client) MkdirAll(remotePath string) (Object, error) {
	user, err := c.UserInfo()
	if err != nil {
		return Object{}, err
	}

	rootObj := Object{
		ObjectKey:  user.RootFolderKey,
		ObjectType: TypeFolder,
		ObjectName: "",
	}

	segments := splitPath(remotePath)
	if len(segments) == 0 {
		return rootObj, nil
	}

	current := rootObj
	for i, seg := range segments {
		if current.ObjectType != TypeFolder {
			return Object{}, fmt.Errorf("segment %q is not a folder", segments[i-1])
		}

		objs, err := c.ListFolder(current.ObjectKey)
		if err != nil {
			return Object{}, fmt.Errorf("listing %s: %w", current.ObjectName, err)
		}

		found := false
		for _, o := range objs {
			if o.ObjectName == seg {
				current = o
				found = true
				break
			}
		}

		if !found {
			newKey, err := c.CreateFolder(seg, current.ObjectKey)
			if err != nil {
				return Object{}, fmt.Errorf("creating folder %q: %w", seg, err)
			}
			current = Object{
				ObjectKey:        newKey,
				ObjectType:       TypeFolder,
				ObjectName:       seg,
				ParentObjectKey:  current.ObjectKey,
				ParentObjectType: TypeFolder,
			}
		}
	}

	return current, nil
}

// splitPath splits a slash-separated remote path into clean segments.
func splitPath(p string) []string {
	var out []string
	for _, seg := range strings.Split(p, "/") {
		seg = strings.TrimSpace(seg)
		if seg != "" && seg != "." {
			out = append(out, seg)
		}
	}
	return out
}
