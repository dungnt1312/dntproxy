package http

import "io"

// jsonReader wraps a byte slice as an io.ReadCloser for JSON payloads.
type jsonReader struct {
	data []byte
	pos  int
}

func newJSONReader(data []byte) io.ReadCloser {
	return &jsonReader{data: data}
}

func (r *jsonReader) Read(p []byte) (int, error) {
	if r.pos >= len(r.data) {
		return 0, io.EOF
	}
	n := copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}

func (r *jsonReader) Close() error {
	return nil
}
