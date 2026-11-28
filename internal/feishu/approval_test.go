package feishu

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"

	"hyla/internal/proto"
)

type fakeApprovalLuna struct {
	calls []struct {
		method string
		params any
	}
}

func (f *fakeApprovalLuna) State() (proto.State, error) { return proto.State{}, nil }

func (f *fakeApprovalLuna) Call(method string, params, out any) error {
	f.calls = append(f.calls, struct {
		method string
		params any
	}{method: method, params: params})
	return nil
}

func (f *fakeApprovalLuna) Kept(id string) (proto.FeishuMessage, error) {
	return proto.FeishuMessage{}, nil
}

func TestBuildApprovalCardJSON(t *testing.T) {
	raw, err := BuildApprovalCardJSON("Merge Guard: feat-x", "flagged `t.Skip`", "mq_123", "img_abc", "asks")
	if err != nil {
		t.Fatalf("BuildApprovalCardJSON failed: %v", err)
	}
	var card map[string]any
	if err := json.Unmarshal([]byte(raw), &card); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(raw, `"img_abc"`) || !strings.Contains(raw, `"mq.approve"`) || !strings.Contains(raw, `"mq.reject"`) {
		t.Errorf("card JSON missing expected image/buttons: %s", raw)
	}

	// Info-only card without laneID has no action buttons.
	infoRaw, err := BuildApprovalCardJSON("Browser OK", "All steps passed", "", "img_abc", "done")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(infoRaw, `"mq.approve"`) {
		t.Errorf("info-only card should not contain approve button: %s", infoRaw)
	}
}

func TestOnMergeQueueAction(t *testing.T) {
	luna := &fakeApprovalLuna{}
	b := New(Config{Luna: luna})

	ev := &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Action: &callback.CallBackAction{
				Value: map[string]any{"hyla": "mq.approve", "lane": "mq_42"},
			},
		},
	}
	resp, err := b.onCardAction(t.Context(), ev)
	if err != nil {
		t.Fatalf("onCardAction failed: %v", err)
	}
	if resp == nil || resp.Toast == nil || resp.Toast.Type != "success" {
		t.Fatalf("unexpected toast: %+v", resp)
	}
	if len(luna.calls) != 1 || luna.calls[0].method != "mq.approve" {
		t.Fatalf("unexpected luna calls: %+v", luna.calls)
	}
	p, ok := luna.calls[0].params.(proto.MergeQueueLaneParams)
	if !ok || p.ID != "mq_42" {
		t.Fatalf("unexpected params: %+v", luna.calls[0].params)
	}
}
