package e2e

import (
	"fmt"
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSearchTools_SharedOrgFiltering covers FR-008/FR-009/FR-011 (specs/006-tools-image-storage,
// specs/003-organizations-administration): SearchTools must include a tool in results (with
// owner.orgs populated) when the searcher shares an org with the tool's owner, and must filter
// it out otherwise.
func TestSearchTools_SharedOrgFiltering(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	toolClient := pb.NewToolServiceClient(client.Conn())

	t.Run("ReturnsToolWhenUsersShareOrg", func(t *testing.T) {
		orgID := db.CreateTestOrg("")
		unique := time.Now().UnixNano()

		ownerID := db.CreateTestUser(fmt.Sprintf("e2e-test-shared-org-owner-%d@test.com", unique), "Shared Org Owner")
		searcherID := db.CreateTestUser(fmt.Sprintf("e2e-test-shared-org-searcher-%d@test.com", unique), "Shared Org Searcher")
		db.AddUserToOrg(ownerID, orgID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(searcherID, orgID, "MEMBER", "ACTIVE", 0)

		metro := fmt.Sprintf("San Diego, CA %d", unique)
		toolID := db.CreateTestToolWithMetro(ownerID, "Shared Tool", metro, 1000)

		// Test: searcher searches without org_id (org_id=0), providing metro
		ctx, cancel := ContextWithUserIDAndTimeout(searcherID, 5*time.Second)
		defer cancel()

		req := &pb.SearchToolsRequest{
			OrganizationId: 0, // Not specified
			Metro:          metro,
			Query:          "Shared",
			Page:           1,
			PageSize:       20,
		}

		resp, err := toolClient.SearchTools(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Find the tool in results
		var foundTool *pb.Tool
		for _, tool := range resp.Tools {
			if tool.Id == toolID {
				foundTool = tool
				break
			}
		}

		require.NotNil(t, foundTool, "Should find tool ID %d in search results", toolID)
		assert.Equal(t, toolID, foundTool.Id)

		// Verify: Owner field should be populated
		require.NotNil(t, foundTool.Owner, "Tool owner should be populated")
		assert.Equal(t, ownerID, foundTool.Owner.Id, "Owner ID should be the tool owner")

		// Verify: Owner should have shared organizations
		require.NotNil(t, foundTool.Owner.Orgs, "Owner orgs should not be nil")
		assert.Greater(t, len(foundTool.Owner.Orgs), 0, "Owner should have at least one shared org")

		// Verify: Shared org should be orgID
		foundSharedOrg := false
		for _, org := range foundTool.Owner.Orgs {
			if org.Id == orgID {
				foundSharedOrg = true
				break
			}
		}
		assert.True(t, foundSharedOrg, "Owner orgs should contain org ID %d", orgID)
	})

	t.Run("FiltersOutToolWhenNoSharedOrg", func(t *testing.T) {
		unique := time.Now().UnixNano()

		// Setup: Create two separate orgs
		org1ID := db.CreateTestOrg("")
		org2ID := db.CreateTestOrg("")

		// Setup: Create users in different orgs
		ownerID := db.CreateTestUser(fmt.Sprintf("e2e-test-noshared-owner-%d@test.com", unique), "Separate Owner")
		searcherID := db.CreateTestUser(fmt.Sprintf("e2e-test-noshared-searcher-%d@test.com", unique), "Separate Searcher")

		db.AddUserToOrg(ownerID, org1ID, "MEMBER", "ACTIVE", 0)    // Owner in org1
		db.AddUserToOrg(searcherID, org2ID, "MEMBER", "ACTIVE", 0) // Searcher in org2 (different!)

		// Setup: Create tool owned by owner in org1's metro
		var org1Metro string
		err := db.QueryRow("SELECT metro FROM orgs WHERE id = $1", org1ID).Scan(&org1Metro)
		require.NoError(t, err)

		toolID := db.CreateTestToolWithMetro(ownerID, "Different Org Tool", org1Metro, 1500)

		// Test: Searcher searches in same metro but different org
		ctx, cancel := ContextWithUserIDAndTimeout(searcherID, 5*time.Second)
		defer cancel()

		req := &pb.SearchToolsRequest{
			OrganizationId: 0, // Not specified
			Metro:          org1Metro,
			Query:          "Different", // Should match tool name
			Page:           1,
			PageSize:       20,
		}

		resp, err := toolClient.SearchTools(ctx, req)
		require.NoError(t, err)
		require.NotNil(t, resp)

		// Verify: Tool should be filtered out (no shared org)
		for _, tool := range resp.Tools {
			assert.NotEqual(t, toolID, tool.Id, "Tool from different org should be filtered out")
		}
	})
}
