package proton

import (
	"context"
	"io"
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

	return c.do(uploadCtx, func(r *resty.Request) (*resty.Response, error) {
		return r.
			SetHeader("pm-storage-token", token).
			SetMultipartField("Block", "blob", "application/octet-stream", block).
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
