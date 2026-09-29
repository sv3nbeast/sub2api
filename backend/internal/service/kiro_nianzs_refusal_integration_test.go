package service

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

// A content-filter stop reaches the client as a completed refusal turn on the
// first account, so the user can follow up instead of seeing an empty reply or
// a replay of the same refused request. KRS names the reason; the Q endpoint
// ends the turn after reasoning without one.
func TestNianzsMessagesDeliversUpstreamRefusalWithoutRetry(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, endpoint := range []struct {
		name     string
		mode     string
		metadata bool
		category string
	}{
		{name: "amazon_q", mode: KiroEndpointModeQ, category: "cyber"},
		{name: "krs", mode: KiroEndpointModeKRS, metadata: true, category: "cyber"},
	} {
		endpoint := endpoint
		for _, stream := range []bool{false, true} {
			stream := stream
			t.Run(endpoint.name+"/"+map[bool]string{false: "non_stream", true: "stream"}[stream], func(t *testing.T) {
				body := []byte(fmt.Sprintf(`{"model":"claude-opus-5","max_tokens":8192,"stream":%t,"thinking":{"type":"adaptive"},"messages":[{"role":"user","content":"verify the finding"}]}`, stream))
				parsed, err := ParseGatewayRequest(NewRequestBodyRef(body), PlatformKiro)
				require.NoError(t, err)
				groupID := int64(29)
				parsed.GroupID = &groupID
				parsed.Group = &Group{ID: groupID, Platform: PlatformKiro, KiroEndpointMode: endpoint.mode}

				upstreamBody := bytes.NewBuffer(nil)
				_, _ = upstreamBody.Write(kiroEventStreamFrame(t, "reasoningContentEvent", map[string]any{"text": "Considering the request."}))
				if endpoint.metadata {
					_, _ = upstreamBody.Write(kiroEventStreamFrame(t, "metadataEvent", map[string]any{
						"stopDetails": map[string]any{"refusal": map[string]any{
							"category":    "CYBER",
							"explanation": "The selected model cannot continue this conversation.",
						}},
						"stopReason": "CONTENT_FILTERED",
					}))
				}
				_, _ = upstreamBody.Write(kiroEventStreamFrame(t, "contextUsageEvent", map[string]any{"contextUsagePercentage": 21.7}))
				svc, upstream, account := newNianzsKiroRouteTestRuntime(t, &http.Response{
					StatusCode: http.StatusOK,
					Header:     http.Header{"Content-Type": []string{"application/vnd.amazon.eventstream"}},
					Body:       io.NopCloser(bytes.NewReader(upstreamBody.Bytes())),
				})
				recorder := httptest.NewRecorder()
				c, _ := gin.CreateTestContext(recorder)
				c.Request = httptest.NewRequest(http.MethodPost, "/v1/messages", bytes.NewReader(body))

				result, forwardErr := svc.Forward(context.Background(), c, account, parsed)

				require.NoError(t, forwardErr)
				require.NotNil(t, result)
				require.Len(t, upstream.requests, 1, "a refused turn must not be replayed")
				stopDetails := gjson.Get(recorder.Body.String(), "stop_details")
				if stream {
					deltas := nianzsSSEPayloadsByType(recorder.Body.String(), "message_delta")
					require.Len(t, deltas, 1)
					require.Equal(t, "refusal", deltas[0].Get("delta.stop_reason").String())
					stopDetails = deltas[0].Get("delta.stop_details")
					require.Equal(t, 1, strings.Count(recorder.Body.String(), "event: message_stop"))
					require.NotContains(t, recorder.Body.String(), "event: error")
				} else {
					require.Equal(t, "refusal", gjson.Get(recorder.Body.String(), "stop_reason").String())
				}
				require.Equal(t, "refusal", stopDetails.Get("type").String())
				require.Equal(t, endpoint.category, stopDetails.Get("category").String())
				require.NotEmpty(t, stopDetails.Get("explanation").String())
			})
		}
	}
}
