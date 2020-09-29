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

func TestMain(m *testing.M) {
	os.Exit(testMain(m))
}

func testMain(m *testing.M) int {
	var err error
	storeConfig, err = cfg.ReadStoreConfig("./postgresStoreConfig.json")
	if err != nil {
		log.Fatal("fail to read config file", err)
	}
	speedleStore, err = store.NewStore(storeConfig.StoreType, storeConfig.StoreProps)
	if err != nil {
		fmt.Println("Could not initialize postgres store ")
		return 1
	}
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

func TestManagePolicies(t *testing.T) {

	speedleStore.DeleteService("service1")

	app := pms.Service{Name: "service1", Type: pms.TypeApplication}
	err := speedleStore.CreateService(&app)

	if err != nil {
		t.Fatal("failed to create application", err)
	}

	var policy pms.Policy
	policy.Name = "policy1"
	policy.Effect = "grant"
	policy.Permissions = []*pms.Permission{
		{
			Resource: "/node1",
			Actions:  []string{"get", "create", "delete"},
		},
	}

	policy.Principals = [][]string{{"user:Alice"}}
	policyR, err := speedleStore.CreatePolicy("service1", &policy)
	if err != nil {
		t.Fatal("failed to create policy: ", err)
	}

	policyR1, err := speedleStore.GetPolicy("service1", policyR.ID)
	t.Log(policyR1)
	if err != nil {
		t.Fatal("failed to get policy: ", err)
	}

	policies, err := speedleStore.ListAllPolicies("service1", "")
	if len(policies) != 1 {
		t.Fatal("should have 1 policy")
	}

	counts, err := speedleStore.GetPolicyAndRolePolicyCounts()
	if err != nil {
		t.Fatal("failed to getCounts", err)
	}

	if counts["service1"].PolicyCount != 1 {
		t.Fatal("incorrect number of policies")
	}
	if counts["service1"].RolePolicyCount != 0 {
		t.Fatal("incorrect number of role policies")
	}

	_, err = speedleStore.GetPolicy("service1", "nonexistantID")
	t.Log(err)

	if err == nil {
		t.Fatal("should have failed to get policy")
	}

	err = speedleStore.DeletePolicy("service1", "nonexistantID")
	t.Log(err)
	if err == nil {
		t.Fatal("should have failed to delete policy")
	}

	err = speedleStore.DeletePolicy("service1", policyR.ID)
	if err != nil {
		t.Fatal("failed to delete policy", err)
	}

}

func TestManageRolePolicies(t *testing.T) {
	//clean the service firstly
	speedleStore.DeleteService("service1")
	app := pms.Service{Name: "service1", Type: pms.TypeApplication}
	err := speedleStore.CreateService(&app)
	if err != nil {
		t.Fatal("fail to create application:", err)
	}
	var rolePolicy pms.RolePolicy
	rolePolicy.Name = "rp1"
	rolePolicy.Effect = "grant"
	rolePolicy.Roles = []string{"role1"}
	rolePolicy.Principals = []string{"user:Alice"}

	policyR, err := speedleStore.CreateRolePolicy("service1", &rolePolicy)
	if err != nil {
		t.Fatal("fail to create role policy:", err)
	}
	policyR1, err := speedleStore.GetRolePolicy("service1", policyR.ID)
	t.Log(policyR1)
	if err != nil {
		t.Fatal("fail to get role policy:", err)
	}

	rolePolicies, err := speedleStore.ListAllRolePolicies("service1", "")
	if err != nil {
		t.Fatal("fail to list role policies:", err)
	}
	if len(rolePolicies) != 1 {
		t.Fatal("should have 1 role policy")
	}

	counts, err := speedleStore.GetPolicyAndRolePolicyCounts()
	if err != nil {
		t.Fatal("Fail to getCounts", err)
	}
	if counts["service1"].PolicyCount != 0 {
		t.Fatal("incorrect policy number")
	}
	if counts["service1"].RolePolicyCount != 1 {
		t.Fatal("incorrect role policy number")
	}

	_, err = speedleStore.GetRolePolicy("service1", "nonexistID")
	t.Log(err)
	if err == nil {
		t.Fatal("should fail to get role policy")
	}

	err = speedleStore.DeleteRolePolicy("service1", "nonexistID")
	t.Log(err)
	if err == nil {
		t.Fatal("should fail to delete role policy")
	}

	err = speedleStore.DeleteRolePolicy("service1", policyR.ID)
	if err != nil {
		t.Fatal("fail to delete role policy:", err)
	}
}

func TestCheckItemsCount(t *testing.T) {

	// clean the services
	speedleStore.DeleteServices()

	// Create service1
	app1 := pms.Service{Name: "service1", Type: pms.TypeApplication}
	err := speedleStore.CreateService(&app1)
	if err != nil {
		t.Fatal("fail to create service:", err)
	}
	// Check service count
	serviceCount, err := speedleStore.GetServiceCount()
	if err != nil {
		t.Fatal("Failed to get service count:", err)
	}
	if serviceCount != 1 {
		t.Fatalf("Service count doesn't match, expected: 1, actual: %d", serviceCount)
	}

	// Create policies
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

	// Create Role Policies
	rolePolicies := []pms.RolePolicy{
		{Name: "p01", Effect: "grant", Principals: []string{"user:user1"}, Roles: []string{"role1"}},
		{Name: "p02", Effect: "grant", Principals: []string{"user:user2"}, Roles: []string{"role2"}},
	}
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

	// Create service2
	app2 := pms.Service{Name: "service2", Type: pms.TypeApplication}
	err = speedleStore.CreateService(&app2)
	if err != nil {
		t.Fatal("fail to create service:", err)
	}
	// Check service count
	serviceCount, err = speedleStore.GetServiceCount()
	if err != nil {
		t.Fatal("Failed to get service count:", err)
	}
	if serviceCount != 2 {
		t.Fatalf("Service count doesn't match, expected: 2, actual: %d", serviceCount)
	}

	// Create policies in service2
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
	counts, err := speedleStore.GetPolicyAndRolePolicyCounts()
	if err != nil {
		t.Fatal("Fail to getCounts", err)
	}
	if (counts["service1"].PolicyCount != int64(len(policies))) ||
		(counts["service2"].PolicyCount != int64(len(policies))) {
		t.Fatal("incorrect policy number")
	}
	if (counts["service1"].RolePolicyCount != int64(len(rolePolicies))) ||
		(counts["service2"].RolePolicyCount != int64(len(rolePolicies))) {
		t.Fatal("incorrect role policy number")
	}
	t.Log(counts)

}
