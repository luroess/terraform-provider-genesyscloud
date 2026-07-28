package integration_action

import (
	"context"
	"errors"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/mypurecloud/platform-client-sdk-go/v193/platformclientv2"
	"github.com/mypurecloud/terraform-provider-genesyscloud/genesyscloud/provider"
)

func TestUpdateFunctionDataActionDraftPreservesPublishedIDAndUsesDraftVersion(t *testing.T) {
	const (
		publishedID  = "custom_-_published"
		draftID      = "custom_-_draft"
		draftVersion = 7
	)

	d := schema.TestResourceDataRaw(t, ResourceIntegrationAction().Schema, map[string]interface{}{
		"name":            "test function",
		"category":        "Function-Data-Actions",
		"integration_id":  "integration-id",
		"secure":          true,
		"contract_input":  `{}`,
		"contract_output": `{}`,
		"function_config": []interface{}{map[string]interface{}{
			"file_path":       "function.zip",
			"handler":         "index.handler",
			"runtime":         "nodejs22.x",
			"timeout_seconds": 15,
		}},
	})
	d.SetId(publishedID)

	zipID := "zip-id"
	calledIDs := make([]string, 0, 5)
	publishedVersion := 0
	proxy := &integrationActionsProxy{
		createIntegrationActionDraftAttr: func(_ context.Context, _ *integrationActionsProxy, action *IntegrationAction) (*IntegrationAction, *platformclientv2.APIResponse, error) {
			if action.Id == nil || *action.Id != publishedID {
				t.Fatalf("draft was not created from published ID: %#v", action.Id)
			}
			calledIDs = append(calledIDs, *action.Id)
			return &IntegrationAction{Id: stringPointer(draftID)}, nil, nil
		},
		uploadIntegrationActionDraftFunctionAttr: func(_ context.Context, _ *integrationActionsProxy, id, _ string) (*platformclientv2.APIResponse, error) {
			calledIDs = append(calledIDs, id)
			return nil, nil
		},
		getIntegrationActionDraftFunctionAttr: func(_ context.Context, _ *integrationActionsProxy, id string) (*platformclientv2.Functionconfig, *platformclientv2.APIResponse, error) {
			calledIDs = append(calledIDs, id)
			return &platformclientv2.Functionconfig{Function: &platformclientv2.Function{ZipId: &zipID}}, nil, nil
		},
		updateIntegrationActionDraftWithFunctionAttr: func(_ context.Context, _ *integrationActionsProxy, id string, _ *platformclientv2.Function) (*platformclientv2.Functionconfig, *platformclientv2.APIResponse, error) {
			calledIDs = append(calledIDs, id)
			return &platformclientv2.Functionconfig{}, nil, nil
		},
		getIntegrationActionDraftByIdAttr: func(_ context.Context, _ *integrationActionsProxy, id string) (*platformclientv2.Action, *platformclientv2.APIResponse, error) {
			calledIDs = append(calledIDs, id)
			version := draftVersion
			return &platformclientv2.Action{Version: &version}, nil, nil
		},
		publishIntegrationActionDraftAttr: func(_ context.Context, _ *integrationActionsProxy, id string, version int) (*platformclientv2.APIResponse, error) {
			calledIDs = append(calledIDs, id)
			publishedVersion = version
			return nil, errors.New("forced publish failure")
		},
	}

	diagnostics := updateFunctionDataActionDraft(context.Background(), d, &provider.ProviderMeta{}, proxy)
	if !diagnostics.HasError() {
		t.Fatal("expected publish failure")
	}
	if got := d.Id(); got != publishedID {
		t.Fatalf("state ID changed after failed publish: got %q, want %q", got, publishedID)
	}
	if publishedVersion != draftVersion {
		t.Fatalf("published version = %d, want exact draft version %d", publishedVersion, draftVersion)
	}
	for i, got := range calledIDs[1:] {
		if got != draftID {
			t.Fatalf("draft operation %d used ID %q, want %q (all calls: %v)", i, got, draftID, calledIDs)
		}
	}
}

func TestFunctionZipIDIsComputedOnly(t *testing.T) {
	zipIDSchema := ResourceIntegrationAction().Schema["function_config"].Elem.(*schema.Resource).Schema["zip_id"]
	if !zipIDSchema.Computed || zipIDSchema.Optional || zipIDSchema.Required {
		t.Fatalf("zip_id must be computed-only: %#v", zipIDSchema)
	}
}

func stringPointer(value string) *string { return &value }
