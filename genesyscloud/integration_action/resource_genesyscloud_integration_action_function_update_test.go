package integration_action

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mypurecloud/platform-client-sdk-go/v179/platformclientv2"
)

func TestUpdateFunctionDataActionDraft_PreservesPublishedIDAndUsesDraftVersion(t *testing.T) {
	resourceSchema := ResourceIntegrationAction().Schema
	d := schema.TestResourceDataRaw(t, resourceSchema, map[string]interface{}{
		"name":            "test-action",
		"category":        "Function Data Actions",
		"integration_id":  "integration-1",
		"secure":          true,
		"contract_input":  `{"type":"object"}`,
		"contract_output": `{"type":"object"}`,
		"function_config": []interface{}{map[string]interface{}{
			"file_path":       "/tmp/test.zip",
			"handler":         "index.handler",
			"runtime":         "nodejs22.x",
			"timeout_seconds": 15,
		}},
	})
	d.SetId("published-id")

	publishedVersion := 7
	publishCalled := false
	publishedWithVersion := 0

	iap := &integrationActionsProxy{
		createIntegrationActionDraftAttr: func(ctx context.Context, p *integrationActionsProxy, actionInput *IntegrationAction) (*IntegrationAction, *platformclientv2.APIResponse, error) {
			if actionInput.Id == nil || *actionInput.Id != "published-id" {
				t.Fatalf("expected draft creation to use published-id, got %#v", actionInput.Id)
			}
			draftID := "draft-id"
			return &IntegrationAction{Id: &draftID}, &platformclientv2.APIResponse{StatusCode: http.StatusOK}, nil
		},
		uploadIntegrationActionDraftFunctionAttr: func(ctx context.Context, p *integrationActionsProxy, actionId string, filePath string) (*platformclientv2.APIResponse, error) {
			if actionId != "draft-id" {
				t.Fatalf("expected upload to use draft-id, got %s", actionId)
			}
			return &platformclientv2.APIResponse{StatusCode: http.StatusOK}, nil
		},
		getIntegrationActionDraftFunctionAttr: func(ctx context.Context, p *integrationActionsProxy, actionId string) (*platformclientv2.Functionconfig, *platformclientv2.APIResponse, error) {
			if actionId != "draft-id" {
				t.Fatalf("expected draft function read to use draft-id, got %s", actionId)
			}
			zipID := "zip-1"
			return &platformclientv2.Functionconfig{Function: &platformclientv2.Function{ZipId: &zipID}}, &platformclientv2.APIResponse{StatusCode: http.StatusOK}, nil
		},
		updateIntegrationActionDraftWithFunctionAttr: func(ctx context.Context, p *integrationActionsProxy, actionId string, updateData *platformclientv2.Function) (*platformclientv2.Functionconfig, *platformclientv2.APIResponse, error) {
			if actionId != "draft-id" {
				t.Fatalf("expected draft function update to use draft-id, got %s", actionId)
			}
			return &platformclientv2.Functionconfig{}, &platformclientv2.APIResponse{StatusCode: http.StatusOK}, nil
		},
		getIntegrationActionDraftByIdAttr: func(ctx context.Context, p *integrationActionsProxy, actionId string) (*platformclientv2.Action, *platformclientv2.APIResponse, error) {
			if actionId != "draft-id" {
				t.Fatalf("expected draft version read to use draft-id, got %s", actionId)
			}
			return &platformclientv2.Action{Version: &publishedVersion}, &platformclientv2.APIResponse{StatusCode: http.StatusOK}, nil
		},
		publishIntegrationActionDraftAttr: func(ctx context.Context, p *integrationActionsProxy, actionId string, version int) (*platformclientv2.APIResponse, error) {
			if actionId != "draft-id" {
				t.Fatalf("expected publish to use draft-id, got %s", actionId)
			}
			publishCalled = true
			publishedWithVersion = version
			return &platformclientv2.APIResponse{StatusCode: http.StatusInternalServerError}, assertErr("publish failed")
		},
	}

	diags := updateFunctionDataActionDraft(context.Background(), d, nil, iap)
	if len(diags) == 0 {
		t.Fatalf("expected diagnostics when publish fails")
	}
	if !publishCalled {
		t.Fatalf("expected publish to be called")
	}
	if publishedWithVersion != publishedVersion {
		t.Fatalf("expected publish version %d, got %d", publishedVersion, publishedWithVersion)
	}
	if d.Id() != "published-id" {
		t.Fatalf("expected resource ID to remain published-id, got %s", d.Id())
	}
}

type assertErr string

func (e assertErr) Error() string {
	return string(e)
}
