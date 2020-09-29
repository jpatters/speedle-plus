package postgres

import (
	"fmt"
	"os"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/teramoby/speedle-plus/api/pms"
	"github.com/teramoby/speedle-plus/pkg/cfg"
	"github.com/teramoby/speedle-plus/pkg/store"
)

var speedleStore pms.PolicyStoreManager
var storeConfig *cfg.StoreConfig
var mockPolicy pms.Policy
var mockRolePolicy pms.RolePolicy

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}

func testMain(m *testing.M) int {
	var err error
	storeConfig, err = cfg.ReadStoreConfig("./postgresStoreConfig.json")
	if err != nil {
		log.Fatal("fail to read config file", err)
	}

	// init the speedle store to use across tests
	speedleStore, err = store.NewStore(storeConfig.StoreType, storeConfig.StoreProps)
	if err != nil {
		fmt.Println("Could not initialize postgres store ")
		return 1
	}

	// init a mock policy to use in multiple tests
	mockPolicy.Name = "policy1"
	mockPolicy.Effect = "grant"
	mockPolicy.Permissions = []*pms.Permission{
		{
			Resource: "/node1",
			Actions:  []string{"get", "create", "delete"},
		},
	}
	mockPolicy.Principals = [][]string{{"user:Alice"}}

	// init a mock role policy to use in multiple tests
	mockRolePolicy.Name = "rp1"
	mockRolePolicy.Effect = "grant"
	mockRolePolicy.Roles = []string{"role1"}
	mockRolePolicy.Principals = []string{"user:Alice"}

	return m.Run()
}

func TestWriteReadPolicyStore(t *testing.T) {

	psr, err := speedleStore.ReadPolicyStore()
	if err != nil {
		t.Fatal("failed to read policy store: ", err)
	} else {
		t.Log(fmt.Sprintf("%d existing services before test", len(psr.Services)))
	}

	var ps pms.PolicyStore
	for i := 0; i < 10; i++ {
		service := pms.Service{Name: fmt.Sprintf("app%d", i), Type: pms.TypeApplication}
		ps.Services = append(ps.Services, &service)
	}

	err = speedleStore.WritePolicyStore(&ps)
	if err != nil {
		t.Fatal("failed to write policy store", err)
	}

	psr, err = speedleStore.ReadPolicyStore()
	if err != nil {
		t.Fatal("fail to read policy store:", err)
	}
	if 10 != len(psr.Services) {
		t.Error("should have 10 applications in the store")
	}
	for _, app := range psr.Services {
		t.Log(app.Name, " ")
	}

}

func createTestService(t *testing.T, serviceName string) {
	app := pms.Service{Name: serviceName, Type: pms.TypeApplication}
	err := speedleStore.CreateService(&app)

	if err != nil {
		t.Fatal("failed to create application: ", err)
	}
}
func TestCreateGetService(t *testing.T) {

	// make sure that the service doesn't already exist
	speedleStore.DeleteService("service1")

	// create the service
	createTestService(t, "service1")

	// check to make sure that we can get the service back from the store
	appResult, err := speedleStore.GetService("service1")

	if err != nil {
		t.Fatal("could not get service back from store", err)
	}

	if pms.TypeApplication != appResult.Type {
		t.Fatal("did not get correct service back from store", err)
	}
}

func TestCreateGetPolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	createPolicyResult, err := speedleStore.CreatePolicy("service1", &mockPolicy)
	if err != nil {
		t.Fatal("failed to create policy", err)
	}

	getPolicyResult, err := speedleStore.GetPolicy("service1", createPolicyResult.ID)
	if err != nil {
		t.Fatal("failed to get policy: ", err)
	}
	if getPolicyResult.Name != "policy1" {
		t.Fatal("did not get right policy from GetPolicy")
	}
}

func TestListAllPolicies(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	speedleStore.CreatePolicy("service1", &mockPolicy)

	policies, err := speedleStore.ListAllPolicies("service1", "")
	if err != nil {
		t.Fatal("failed to list all policies", err)
	}

	if len(policies) != 1 {
		t.Fatal("listing policies should only return 1 policy")
	}
}

func TestListAllPoliciesWithFilter(t *testing.T) {
	// implement
}

func TestGetPolicyandRolePolicyCounts(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	speedleStore.CreatePolicy("service1", &mockPolicy)
	speedleStore.CreateRolePolicy("service1", &mockRolePolicy)

	counts, err := speedleStore.GetPolicyAndRolePolicyCounts()
	if err != nil {
		t.Fatal("failed to getCounts", err)
	}

	if counts["service1"].PolicyCount != 1 {
		t.Fatal("incorrect number of policies")
	}
	if counts["service1"].RolePolicyCount != 1 {
		t.Fatal("incorrect number of role policies")
	}
}

func TestErrorOnGetMissingPolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	speedleStore.CreatePolicy("service1", &mockPolicy)

	_, err := speedleStore.GetPolicy("service1", "doesnotexist")
	if err == nil {
		t.Fatal("should have failed getting non-existing policy")
	}

}

func TestErrorOnDeletingMissingPolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	speedleStore.CreatePolicy("service1", &mockPolicy)

	err := speedleStore.DeletePolicy("service1", "doesnotexist")
	if err == nil {
		t.Fatal("should have failed deleting non-existing policy")
	}
}

func TestDeletePolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	createPolicyResult, _ := speedleStore.CreatePolicy("service1", &mockPolicy)
	err := speedleStore.DeletePolicy("service1", createPolicyResult.ID)
	if err != nil {
		t.Fatal("failed to delete policy", err)
	}
}

func TestCreateGetRolePolicies(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	createPolicyResult, err := speedleStore.CreateRolePolicy("service1", &mockRolePolicy)

	if err != nil {
		t.Fatal("failed to create the role policy", err)
	}

	getPolicyResult, err := speedleStore.GetRolePolicy("service1", createPolicyResult.ID)
	if err != nil {
		t.Fatal("failed to get the role policy from the store", err)
	}

	if getPolicyResult.Name != "rp1" {
		t.Fatal("got the wrong role policy from the store")
	}
}

func TestListRolePolicies(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	_, err := speedleStore.CreateRolePolicy("service1", &mockRolePolicy)
	rolePolicies, err := speedleStore.ListAllRolePolicies("service1", "")
	if err != nil {
		t.Fatal("fail to list role policies:", err)
	}
	if len(rolePolicies) != 1 {
		t.Fatal("should have 1 role policy")
	}
}

func TestListRolePoliciesWithFilter(t *testing.T) {
	// implement
}

func TestErrorOnGetMissingRolePolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")

	_, err := speedleStore.GetRolePolicy("service1", "nonexistID")
	if err == nil {
		t.Fatal("should fail to get role policy")
	}

}

func TestDeleteRolePolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	createdRolePolicy, _ := speedleStore.CreateRolePolicy("service1", &mockRolePolicy)

	// make sure that it exists
	rolePolicies, _ := speedleStore.ListAllRolePolicies("service1", "")
	if len(rolePolicies) != 1 {
		t.Fatal("should have 1 role policy")
	}

	// now delete it
	err := speedleStore.DeleteRolePolicy("service1", createdRolePolicy.ID)
	if err != nil {
		t.Fatal("could not delete role policy")
	}

	rolePolicies, _ = speedleStore.ListAllRolePolicies("service1", "")
	if len(rolePolicies) != 0 {
		t.Fatal("should have 0 role policies")
	}
}

func TestErrorOnDeleteMissingRolePolicy(t *testing.T) {
	speedleStore.DeleteService("service1")
	createTestService(t, "service1")
	err := speedleStore.DeleteRolePolicy("service1", "nonexistID")
	if err == nil {
		t.Fatal("should fail to delete role policy")
	}

}

func TestGetServiceCount(t *testing.T) {
	speedleStore.DeleteServices()
	createTestService(t, "service1")
	createTestService(t, "service2")
	serviceCount, err := speedleStore.GetServiceCount()
	if err != nil {
		t.Fatal("failed to get a count of services")
	}

	if serviceCount != 2 {
		t.Fatalf("Service count doesn't match, expected: 2, actual: %d", serviceCount)
	}
}

func TestGetPolicyCount(t *testing.T) {
	speedleStore.DeleteServices()
	createTestService(t, "service1")
	createTestService(t, "service2")

	// Create Policies in service 1
	policies := []pms.Policy{
		{Name: "p01", Effect: "grant", Principals: [][]string{{"user:user1"}}},
		{Name: "p02", Effect: "grant", Principals: [][]string{{"user:user2"}}},
		{Name: "p03", Effect: "grant", Principals: [][]string{{"user:user3"}}},
	}
	for _, policy := range policies {
		_, err := speedleStore.CreatePolicy("service1", &policy)
		if err != nil {
			t.Fatal("fail to create policy:", err)
		}
	}
	// Check policy count
	policyCount, err := speedleStore.GetPolicyCount("service1")
	if err != nil {
		t.Fatal("Failed to get the policy count: ", err)
	}
	if policyCount != int64(len(policies)) {
		t.Fatalf("Policy count doesn't match, expected:%d, actual:%d", len(policies), policyCount)
	}

	// create policies in service 2
	for _, policy := range policies {
		_, err := speedleStore.CreatePolicy("service2", &policy)
		if err != nil {
			t.Fatal("fail to create policy:", err)
		}
	}
	// Check policy count in service2
	policyCount, err = speedleStore.GetPolicyCount("service2")
	if err != nil {
		t.Fatal("Failed to get the policy count: ", err)
	}
	if policyCount != int64(len(policies)) {
		t.Fatalf("Policy count doesn't match, expected:%d, actual:%d", len(policies), policyCount)
	}

	// Check policy count in both service1 and service2
	policyCount, err = speedleStore.GetPolicyCount("")
	if err != nil {
		t.Fatal("Failed to get the policy count: ", err)
	}
	if policyCount != int64(len(policies)*2) {
		t.Fatalf("Policy count doesn't match, expected:%d, actual:%d", len(policies)*2, policyCount)
	}
}

func TestGetRolePolicyCount(t *testing.T) {
	speedleStore.DeleteServices()
	createTestService(t, "service1")
	createTestService(t, "service2")

	rolePolicies := []pms.RolePolicy{
		{Name: "p01", Effect: "grant", Principals: []string{"user:user1"}, Roles: []string{"role1"}},
		{Name: "p02", Effect: "grant", Principals: []string{"user:user2"}, Roles: []string{"role2"}},
	}

	// Create role policies in service 1
	for _, rolePolicy := range rolePolicies {
		_, err := speedleStore.CreateRolePolicy("service1", &rolePolicy)
		if err != nil {
			t.Fatal("Failed to get role policy count:", err)
		}
	}
	// Check role Policy count
	rolePolicyCount, err := speedleStore.GetRolePolicyCount("service1")
	if err != nil {
		t.Fatal("Failed to get the role policy count")
	}
	if rolePolicyCount != int64(len(rolePolicies)) {
		t.Fatalf("RolePolicy count doesn't match, expected:%d, actual:%d", len(rolePolicies), rolePolicyCount)
	}

	// Create rolePolicy in service2
	for _, rolePolicy := range rolePolicies {
		_, err := speedleStore.CreateRolePolicy("service2", &rolePolicy)
		if err != nil {
			t.Fatal("Failed to get role policy count:", err)
		}
	}
	// Check role Policy count in service2
	rolePolicyCount, err = speedleStore.GetRolePolicyCount("service2")
	if err != nil {
		t.Fatal("Failed to get the role policy count")
	}
	if rolePolicyCount != int64(len(rolePolicies)) {
		t.Fatalf("RolePolicy count doesn't match, expected:%d, actual:%d", len(rolePolicies), rolePolicyCount)
	}
	// Check role Policy count in both service1 and service2
	rolePolicyCount, err = speedleStore.GetRolePolicyCount("")
	if err != nil {
		t.Fatal("Failed to get the role policy count")
	}
	if rolePolicyCount != int64(len(rolePolicies)*2) {
		t.Fatalf("RolePolicy count doesn't match, expected:%d, actual:%d", len(rolePolicies)*2, rolePolicyCount)
	}
}
