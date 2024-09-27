package aiproxy

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPing(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case API_PING:
			require.Equal(t, r.Method, "POST")
			require.Equal(t, r.Header.Get("Content-Type"), "application/json")
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)

			var request RequestBody
			err = json.Unmarshal(body, &request)
			require.NoError(t, err)
			require.Equal(t, request.LlmServer[0].Baseurl, "https://api.deepseek.com")
			require.Equal(t, request.LlmServer[0].Ability, "chat")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		default:
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte(`{"status":"fail", error:"unexpected path"}`))
		}
	}))
	defer server.Close()

	proxy := AIProxy{
		Url: server.URL,
		Servers: []Server{
			{
				Baseurl: "https://api.deepseek.com",
				Apikey:  "123456",
				Model:   "deepseek-coder",
				Ability: "chat",
			},
		},
		Timeout: 0,
	}
	if err := proxy.Ping(); err != nil {
		t.Errorf("ping failed: %v", err)
	}
}
