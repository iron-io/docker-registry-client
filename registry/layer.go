package registry

import (
	_ "crypto/sha256"
	_ "crypto/sha512"
	"io"
	"net/http"
	"net/url"

	"github.com/opencontainers/go-digest"
)

func (registry *Registry) DownloadLayer(repository string, digest digest.Digest) (io.ReadCloser, error) {
	url := registry.url("/v2/%s/blobs/%s", repository, digest)
	registry.Logf("registry.layer.download url=%s repository=%s digest=%s", url, repository, digest)

	resp, err := registry.Client.Get(url)
	if err != nil {
		return nil, err
	}

	return resp.Body, nil
}

func (registry *Registry) UploadLayer(repository string, digest digest.Digest, content io.Reader) error {
	uploadUrl, err := registry.initiateUpload(repository)
	if err != nil {
		return err
	}
	q := uploadUrl.Query()
	q.Set("digest", digest.String())
	uploadUrl.RawQuery = q.Encode()

	registry.Logf("registry.layer.upload url=%s repository=%s digest=%s", uploadUrl, repository, digest)

	upload, err := http.NewRequest("PUT", uploadUrl.String(), content)
	if err != nil {
		return err
	}
	upload.Header.Set("Content-Type", "application/octet-stream")

	_, err = registry.Client.Do(upload)
	return err
}

func (registry *Registry) HasLayer(repository string, digest digest.Digest) (bool, error) {
	checkUrl := registry.url("/v2/%s/blobs/%s", repository, digest)
	registry.Logf("registry.layer.check url=%s repository=%s digest=%s", checkUrl, repository, digest)

	resp, err := registry.Client.Head(checkUrl)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		return resp.StatusCode == http.StatusOK, nil
	}

	urlErr, ok := err.(*url.Error)
	if !ok {
		return false, err
	}
	httpErr, ok := urlErr.Err.(*HttpStatusError)
	if !ok {
		return false, err
	}
	if httpErr.Response.StatusCode == http.StatusNotFound {
		return false, nil
	}

	return false, err
}

// taken from https://github.com/docker/distribution/blob/06fa77aa11a3913096efcb9b5bd25db8ef55a939/blobs.go

// Descriptor describes targeted content. Used in conjunction with a blob
// store, a descriptor can be used to fetch, store and target any kind of
// blob. The struct also describes the wire protocol format. Fields should
// only be added but never changed.
type Descriptor struct {
	// MediaType describe the type of the content. All text based formats are
	// encoded as utf-8.
	MediaType string `json:"mediaType,omitempty"`

	// Size in bytes of content.
	Size int64 `json:"size,omitempty"`

	// Digest uniquely identifies the content. A byte stream can be verified
	// against against this digest.
	Digest digest.Digest `json:"digest,omitempty"`

	// URLs contains the source URLs of this content.
	URLs []string `json:"urls,omitempty"`

	// NOTE: Before adding a field here, please ensure that all
	// other options have been exhausted. Much of the type relationships
	// depend on the simplicity of this type.
}

func (registry *Registry) LayerMetadata(repository string, digest digest.Digest) (Descriptor, error) {
	checkUrl := registry.url("/v2/%s/blobs/%s", repository, digest)
	registry.Logf("registry.layer.check url=%s repository=%s digest=%s", checkUrl, repository, digest)

	resp, err := registry.Client.Head(checkUrl)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return distribution.Descriptor{}, err
	}

	return Descriptor{
		Digest: digest,
		Size:   resp.ContentLength,
	}, nil
}

func (registry *Registry) initiateUpload(repository string) (*url.URL, error) {
	initiateUrl := registry.url("/v2/%s/blobs/uploads/", repository)
	registry.Logf("registry.layer.initiate-upload url=%s repository=%s", initiateUrl, repository)

	resp, err := registry.Client.Post(initiateUrl, "application/octet-stream", nil)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err != nil {
		return nil, err
	}

	location := resp.Header.Get("Location")
	locationUrl, err := url.Parse(location)
	if err != nil {
		return nil, err
	}
	return locationUrl, nil
}
