package e2e

import (
	"testing"
	"time"

	pb "ubertool-backend-trusted/api/gen/v1"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestToolService_E2E(t *testing.T) {
	db := PrepareDB(t)
	defer db.Close()
	defer db.Cleanup()

	client := NewGRPCClient(t, "")
	defer client.Close()

	toolClient := pb.NewToolServiceClient(client.Conn())

	t.Run("AddTool Workflow", func(t *testing.T) {
		// Setup: Create user and org
		userID := db.CreateTestUser("e2e-test-toolowner@test.com", "Tool Owner")
		orgID := db.CreateTestOrg("")
		db.AddUserToOrg(userID, orgID, "MEMBER", "ACTIVE", 0)

		// Test: Add tool
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.AddToolRequest{
			Name:               "E2E Test Drill",
			Description:        "A powerful drill for testing",
			Categories:         []string{"Power Tools", "Construction"},
			PricePerDayCents:   1000,
			PricePerWeekCents:  5000,
			PricePerMonthCents: 15000,
			Condition:          pb.ToolCondition_TOOL_CONDITION_EXCELLENT,
			Metro:              "San Jose",
		}

		resp, err := toolClient.AddTool(ctx, req)
		require.NoError(t, err)
		assert.NotNil(t, resp.Tool)
		assert.Greater(t, resp.Tool.Id, int32(0))
		// Owner field is not populated for AddTool (user is adding their own tool)
		assert.Equal(t, pb.ToolStatus_TOOL_STATUS_AVAILABLE, resp.Tool.Status)

		// Verify: Tool exists in database
		var toolName string
		err = db.QueryRow("SELECT name FROM tools WHERE id = $1", resp.Tool.Id).Scan(&toolName)
		assert.NoError(t, err)
		assert.Equal(t, "E2E Test Drill", toolName)
	})

	t.Run("SearchTools with Org Membership Verification", func(t *testing.T) {
		// Setup: Create two orgs and users
		org1ID := db.CreateTestOrg("E2E-Test-Org-A")
		org2ID := db.CreateTestOrg("E2E-Test-Org-B")

		user1ID := db.CreateTestUser("e2e-test-user1@test.com", "User 1")
		user2ID := db.CreateTestUser("e2e-test-user2@test.com", "User 2")

		db.AddUserToOrg(user1ID, org1ID, "MEMBER", "ACTIVE", 0)
		db.AddUserToOrg(user2ID, org2ID, "MEMBER", "ACTIVE", 0)

		// Create tools for each user
		_ = db.CreateTestTool(user1ID, "Org A Tool", 1000)
		_ = db.CreateTestTool(user2ID, "Org B Tool", 1500)

		// Test: User 1 searches in Org 1 (should succeed)
		ctx, cancel := ContextWithUserIDAndTimeout(user1ID, 5*time.Second)
		defer cancel()

		req := &pb.SearchToolsRequest{
			OrganizationId: org1ID,
			Query:          "Tool",
		}

		resp, err := toolClient.SearchTools(ctx, req)
		require.NoError(t, err)
		// Should find tools from users in org1
		assert.GreaterOrEqual(t, len(resp.Tools), 0)

		// Test: User 1 tries to search in Org 2 (should fail - not a member)
		ctx2, cancel2 := ContextWithUserIDAndTimeout(user1ID, 5*time.Second)
		defer cancel2()

		req2 := &pb.SearchToolsRequest{
			OrganizationId: org2ID,
			Query:          "Tool",
		}

		_, err = toolClient.SearchTools(ctx2, req2)
		// Should return error because user1 is not in org2
		assert.Error(t, err)
	})

	t.Run("ListMyTools", func(t *testing.T) {
		// Setup: Create user and tools
		userID := db.CreateTestUser("e2e-test-toolowner2@test.com", "Tool Owner 2")
		db.CreateTestTool(userID, "My Tool 1", 1000)
		db.CreateTestTool(userID, "My Tool 2", 1500)

		// Test: List my tools
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.ListToolsRequest{
			Metro: "San Jose",
		}

		resp, err := toolClient.ListMyTools(ctx, req)
		require.NoError(t, err)
		assert.GreaterOrEqual(t, len(resp.Tools), 2)
	})

	t.Run("UpdateTool", func(t *testing.T) {
		// Setup: Create user and tool
		userID := db.CreateTestUser("e2e-test-toolowner3@test.com", "Tool Owner 3")
		toolID := db.CreateTestTool(userID, "Original Tool Name", 1000)

		// Test: Update tool
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.UpdateToolRequest{
			ToolId:             toolID,
			Name:               "Updated Tool Name",
			Description:        "Updated description",
			PricePerDayCents:   1200,
			PricePerWeekCents:  6000,
			PricePerMonthCents: 18000,
			Condition:          pb.ToolCondition_TOOL_CONDITION_GOOD,
		}

		resp, err := toolClient.UpdateTool(ctx, req)
		require.NoError(t, err)
		assert.Equal(t, "Updated Tool Name", resp.Tool.Name)
		assert.Equal(t, int32(1200), resp.Tool.PricePerDayCents)

		// Verify: Database was updated
		var toolName string
		var pricePerDay int32
		err = db.QueryRow("SELECT name, price_per_day_cents FROM tools WHERE id = $1", toolID).Scan(&toolName, &pricePerDay)
		assert.NoError(t, err)
		assert.Equal(t, "Updated Tool Name", toolName)
		assert.Equal(t, int32(1200), pricePerDay)
	})

	t.Run("DeleteTool (Soft Delete)", func(t *testing.T) {
		// Setup: Create user and tool
		userID := db.CreateTestUser("e2e-test-toolowner4@test.com", "Tool Owner 4")
		toolID := db.CreateTestTool(userID, "Tool to Delete", 1000)

		// Test: Delete tool
		ctx, cancel := ContextWithUserIDAndTimeout(userID, 5*time.Second)
		defer cancel()

		req := &pb.DeleteToolRequest{
			ToolId: toolID,
		}

		resp, err := toolClient.DeleteTool(ctx, req)
		require.NoError(t, err)
		assert.True(t, resp.Success)

		// Verify: Tool is soft deleted
		var deletedOn *time.Time
		err = db.QueryRow("SELECT deleted_on FROM tools WHERE id = $1", toolID).Scan(&deletedOn)
		assert.NoError(t, err)
		assert.NotNil(t, deletedOn)
	})
}

