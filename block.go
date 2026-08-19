package proton

import (
	"bytes"
	"context"
	"io"
	"mime/multipart"
	"time"

	"github.com/go-resty/resty/v2"
)

func (c *Client) GetBlock(ctx context.Context, bareURL, token string) (io.ReadCloser, error) {
	res, err := c.doRes(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetHeader("pm-storage-token", token).SetDoNotParseResponse(true).Get(bareURL)
	})
	if err != nil {
		return nil, err
	}

	return res.RawBody(), nil
}

func (c *Client) RequestBlockUpload(ctx context.Context, req BlockUploadReq) ([]BlockUploadLink, error) {
	var res struct {
		UploadLinks []BlockUploadLink
	}

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).SetBody(req).Post("/drive/blocks")
	}); err != nil {
		return nil, err
	}

	return res.UploadLinks, nil
}

func (c *Client) UploadBlock(ctx context.Context, bareURL, token string, block io.Reader) error {
	// Per-block upload timeout: 5 minutes is a reasonable upper bound for a 4MB block
	// This prevents a single hung connection from blocking the entire upload pipeline
	// and from holding authLock.RLock() indefinitely (which would block auth refresh for all requests)
	uploadCtx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	// Pre-encode the multipart body into memory so retries replay the complete
	// body. Handing the raw reader to SetMultipartField means any resty retry
	// after a network error (e.g. timeout awaiting response headers) re-sends
	// an exhausted reader, which the server rejects with
	// 400 "Upload file empty (Code=2003)".
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	fw, err := w.CreateFormFile("Block", "blob")
	if err != nil {
		return err
	}
	if _, err := io.Copy(fw, block); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}

	return c.do(uploadCtx, func(r *resty.Request) (*resty.Response, error) {
		return r.
			SetHeader("pm-storage-token", token).
			SetHeader("Content-Type", w.FormDataContentType()).
			SetBody(buf.Bytes()).
			Post(bareURL)
	})
}

func (c *Client) Verification(ctx context.Context, shareID string, linkID string, revision string) (VerificationRes, error) {
	var res VerificationRes

	if err := c.do(ctx, func(r *resty.Request) (*resty.Response, error) {
		return r.SetResult(&res).Get("/drive/shares/" + shareID + "/links/" + linkID + "/revisions/" + revision + "/verification")
	}); err != nil {
		return res, err
	}
	return res, nil
}
