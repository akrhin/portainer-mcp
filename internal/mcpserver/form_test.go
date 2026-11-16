package mcpserver

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"
)

// TestFormBody covers the multipart-only operations: the body object is sent as
// a form, a property naming a filename becomes a file part, and everything else
// travels as text Portainer can parse (TagIds=[1,2], Env=[{"name":…}]).
func TestFormBody(t *testing.T) {
	var values map[string]string
	var filename, content string
	h := func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); !strings.HasPrefix(ct, "multipart/form-data") {
			t.Errorf("Content-Type = %q", ct)
			return
		}
		if err := r.ParseMultipartForm(1 << 20); err != nil {
			t.Errorf("ParseMultipartForm: %v", err)
			return
		}
		values = map[string]string{}
		for name, v := range r.MultipartForm.Value {
			values[name] = v[0]
		}
		f, hdr, err := r.FormFile("file")
		if err != nil {
			t.Errorf("file part: %v", err)
			return
		}
		defer f.Close()
		raw, _ := io.ReadAll(f)
		filename, content = hdr.Filename, string(raw)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"Id":1}`))
	}

	res, hits := testCall(t, h, []string{"core", "docker", "stacks"}, "stack_create_docker_standalone_file", map[string]any{
		"endpointId": float64(7),
		"body": map[string]any{
			"Name":   "web",
			"TagIds": []any{float64(1), float64(2)},
			"Limit":  float64(3),
			"Public": true,
			"Env":    []any{map[string]any{"name": "MODE", "value": "prod"}},
			"file":   map[string]any{"filename": "stack.yml", "content": "services: {}\n"},
		},
	})
	if res.IsError {
		t.Fatalf("form tool = %q", text(t, res))
	}
	if hits != 1 {
		t.Fatalf("requests = %d, want 1", hits)
	}
	for name, want := range map[string]string{
		"Name":   "web",
		"TagIds": "[1,2]",
		"Limit":  "3",
		"Public": "true",
		"Env":    `[{"name":"MODE","value":"prod"}]`,
	} {
		if got := values[name]; got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if filename != "stack.yml" || content != "services: {}\n" {
		t.Fatalf("file part = %q %q", filename, content)
	}

	// A file may arrive base64-encoded instead of as text.
	res, _ = testCall(t, h, []string{"core", "docker", "stacks"}, "stack_create_docker_standalone_file", map[string]any{
		"endpointId": float64(7),
		"body": map[string]any{
			"Name": "web",
			"file": map[string]any{"filename": "a.txt", "base64": base64.StdEncoding.EncodeToString([]byte("raw\000"))},
		},
	})
	if res.IsError {
		t.Fatalf("base64 form tool = %q", text(t, res))
	}
	if filename != "a.txt" || content != "raw\000" {
		t.Fatalf("base64 file part = %q %q", filename, content)
	}
}

// TestContainerExecRuns covers the second defect: the tool creates the exec
// instance and starts it, so the caller gets the command's output instead of an
// id, and a multiplexed stream is decoded.
func TestContainerExecRuns(t *testing.T) {
	var startBody string
	var paths []string
	res, _ := testCall(t, func(w http.ResponseWriter, r *http.Request) {
		paths = append(paths, r.URL.Path)
		raw, _ := io.ReadAll(r.Body)
		switch {
		case strings.HasSuffix(r.URL.Path, "/containers/abc/exec"):
			var body map[string]any
			if err := json.Unmarshal(raw, &body); err != nil {
				t.Errorf("create body %q: %v", raw, err)
			}
			if _, ok := body["Cmd"]; !ok {
				t.Errorf("create body lost Cmd: %s", raw)
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"Id":"abc"}`))
		case strings.HasSuffix(r.URL.Path, "/exec/abc/start"):
			startBody = string(raw)
			payload := []byte("hello\n")
			frame := make([]byte, 8, 8+len(payload))
			frame[0] = 1 // stdout
			binary.BigEndian.PutUint32(frame[4:], uint32(len(payload)))
			_, _ = w.Write(append(frame, payload...))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}, []string{"core", "docker"}, "container_exec", map[string]any{
		"environment": float64(7),
		"id":          "abc",
		"body":        map[string]any{"Cmd": []any{"echo", "hello"}, "AttachStdout": true},
	})
	if res.IsError {
		t.Fatalf("container_exec = %q", text(t, res))
	}
	if got := text(t, res); got != "hello\n" {
		t.Fatalf("output = %q, want %q", got, "hello\n")
	}
	if want := "/api/endpoints/7/docker/containers/abc/exec"; paths[0] != want {
		t.Fatalf("first call = %s, want %s", paths[0], want)
	}
	if want := "/api/endpoints/7/docker/exec/abc/start"; len(paths) != 2 || paths[1] != want {
		t.Fatalf("calls = %v, want the start at %s", paths, want)
	}
	if !strings.Contains(startBody, `"Detach":false`) || !strings.Contains(startBody, `"Tty":false`) {
		t.Fatalf("start body = %s", startBody)
	}
}

// TestContainerExecRefusesStdin: the tool sends no stdin, so an attached stdin
// would hand the command an immediate EOF and waste the call.
func TestContainerExecRefusesStdin(t *testing.T) {
	res, hits := testCall(t, jsonBody(`{"Id":"abc"}`), []string{"core", "docker"}, "container_exec", map[string]any{
		"environment": float64(7),
		"id":          "abc",
		"body":        map[string]any{"Cmd": []any{"cat"}, "AttachStdin": true},
	})
	if !res.IsError || !strings.Contains(text(t, res), "AttachStdin is not supported") {
		t.Fatalf("refusal = %q", text(t, res))
	}
	if hits != 0 {
		t.Fatalf("a refused call still reached the API (%d requests)", hits)
	}
}

// TestUntilAcceptsEpoch keeps the timestamp escape hatch: until as a JSON
// number bounds the stream, so the follow flag no longer refuses the call.
func TestUntilAcceptsEpoch(t *testing.T) {
	sets := []string{"core", "docker"}

	res, hits := testCall(t, jsonBody(`[]`), sets, "container_logs", map[string]any{
		"environment": float64(7), "id": "abc", "follow": true, "until": float64(1700000000),
	})
	if res.IsError {
		t.Fatalf("bounded follow = %q", text(t, res))
	}
	if hits != 1 {
		t.Fatalf("bounded follow sent %d requests, want 1", hits)
	}

	res, hits = testCall(t, jsonBody(`[]`), sets, "container_logs", map[string]any{
		"environment": float64(7), "id": "abc", "follow": true,
	})
	if !res.IsError || hits != 0 {
		t.Fatalf("unbounded follow = error %v after %d requests", res.IsError, hits)
	}
}
