package postgres

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"github.com/teramoby/speedle-plus/api/pms"
	"github.com/xtgo/uuid"
)

type Store struct {
	client      *sqlx.DB
	tablePrefix string
}

type RowScanner interface {
	Scan(dest ...interface{}) error
}

type ChangeEvent struct {
	Table   string      `json:"table"`
	Action  string      `json:"action"`
	Service pms.Service `json:"data"`
}

func (s *Store) CreateService(service *pms.Service) error {
	query := fmt.Sprintf("INSERT INTO %s (name, type, policies, role_policies, metadata) VALUES($1, $2, $3, $4, $5)", s.prefixedTable("services"))

	if len(service.Policies) == 0 {
		service.Policies = make([]*pms.Policy, 0)
	}

	if len(service.RolePolicies) == 0 {
		service.RolePolicies = make([]*pms.RolePolicy, 0)
	}

	policies, err := json.Marshal(service.Policies)
	if err != nil {
		return err
	}

	rolePolicies, err := json.Marshal(service.RolePolicies)
	if err != nil {
		return err
	}

	metadata, err := json.Marshal(service.Metadata)
	if err != nil {
		return err
	}

	result, err := s.client.Exec(query, service.Name, service.Type, policies, rolePolicies, metadata)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		return errors.New("unable to create service; an unknown error occurred")
	}

	return nil
}

func (s *Store) DeleteService(serviceName string) error {
	query := fmt.Sprintf("DELETE FROM %s WHERE name = $1", s.prefixedTable("services"))
	result, err := s.client.Exec(query, serviceName)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		return errors.New("unable to delete service; service does not exist")
	}

	return nil
}

func (s *Store) DeleteServices() error {
	query := fmt.Sprintf("DELETE FROM %s", s.prefixedTable("services"))
	result, err := s.client.Exec(query)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		log.Info("No rows to delete")
	}

	return nil
}

func scanService(row RowScanner, service *pms.Service) error {
	var name, t string
	var rawPolicies, rawRolePolicies, rawMetadata []byte
	err := row.Scan(&name, &t, &rawPolicies, &rawRolePolicies, &rawMetadata)
	if err != nil {
		return err
	}

	var policies []*pms.Policy
	var rolePolicies []*pms.RolePolicy
	var metadata map[string]string

	err = json.Unmarshal(rawPolicies, &policies)
	if err != nil {
		return err
	}

	err = json.Unmarshal(rawRolePolicies, &rolePolicies)
	if err != nil {
		return err
	}

	err = json.Unmarshal(rawMetadata, &metadata)
	if err != nil {
		return err
	}

	service.Name = name
	service.Type = t
	service.Policies = policies
	service.RolePolicies = rolePolicies
	service.Metadata = metadata

	return nil
}

func (s *Store) GetService(serviceName string) (*pms.Service, error) {
	query := fmt.Sprintf("SELECT name, type, policies, role_policies, metadata FROM %s WHERE name = $1", s.prefixedTable("services"))
	row := s.client.QueryRow(query, serviceName)
	service := &pms.Service{}
	err := scanService(row, service)
	return service, err
}

func (s *Store) ListAllServices() ([]*pms.Service, error) {
	query := fmt.Sprintf("SELECT name, type, policies, role_policies, metadata FROM %s", s.prefixedTable("services"))
	rows, err := s.client.Query(query)
	if err != nil {
		return nil, err
	}

	services := []*pms.Service{}
	for rows.Next() {
		service := &pms.Service{}
		err := scanService(rows, service)
		if err != nil {
			return nil, err
		}

		services = append(services, service)
	}
	return services, err
}

func (s *Store) GetServiceCount() (int64, error) {
	query := fmt.Sprintf("SELECT count(1) AS ct FROM %s", s.prefixedTable("services"))
	row := s.client.QueryRow(query)
	var count int64
	err := row.Scan(&count)
	return count, err
}

func (s *Store) GetServiceNames() ([]string, error) {
	query := fmt.Sprintf("SELECT name FROM %s", s.prefixedTable("services"))
	rows, err := s.client.Query(query)
	result := []string{}
	if err != nil {
		return result, err
	}

	for rows.Next() {
		var name string
		rows.Scan(&name)
		result = append(result, name)
	}

	return result, nil
}

func (s *Store) GetPolicyAndRolePolicyCounts() (map[string]*pms.PolicyAndRolePolicyCount, error) {
	query := fmt.Sprintf("SELECT name, jsonb_aray_length(policies) as policy_count, jsonb_array_length(role_policies) as role_policy_count FROM %s", s.prefixedTable("services"))
	rows, err := s.client.Query(query)
	if err != nil {
		return nil, err
	}

	result := map[string]*pms.PolicyAndRolePolicyCount{}
	for rows.Next() {
		var name string
		var policyCount, rolePolicyCount int64
		err := rows.Scan(&name, &policyCount, &rolePolicyCount)
		if err != nil {
			return nil, err
		}

		result[name] = &pms.PolicyAndRolePolicyCount{
			PolicyCount:     policyCount,
			RolePolicyCount: rolePolicyCount,
		}
	}

	return result, nil
}

func (s *Store) ReadPolicyStore() (*pms.PolicyStore, error) {
	var ps pms.PolicyStore
	services, err := s.ListAllServices()
	if err != nil {
		return nil, err
	}
	ps.Services = services

	ps.Functions = nil

	return &ps, nil

}

func (s *Store) WritePolicyStore(ps *pms.PolicyStore) error {
	err := s.DeleteServices()
	if err != nil {
		return err
	}

	for _, service := range ps.Services {
		err := s.CreateService(service)
		if err != nil {
			return err
		}
	}

	for _, function := range ps.Functions {
		_, err := s.CreateFunction(function)
		if err == nil {
			return err
		}
	}
	return nil
}

func (s *Store) Type() string {
	return StoreType
}

func (s *Store) CreatePolicy(serviceName string, policy *pms.Policy) (*pms.Policy, error) {
	policy.ID = uuid.NewRandom().String()
	jsonPolicy, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("UPDATE %s SET policies = policies || $1::jsonb WHERE name = $2", s.prefixedTable("services"))
	result, err := s.client.Exec(query, jsonPolicy, serviceName)
	if err != nil {
		return nil, err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowCount == 0 {
		return nil, errors.New("unable to create policy; an unknown error occurred")
	}
	return policy, nil
}

func (s *Store) DeletePolicy(serviceName string, id string) error {
	query := fmt.Sprintf(`
	UPDATE %s SET policies = policies -
	CAST((
		SELECT position - 1
		FROM %s, jsonb_array_elements(policies) with ordinality arr(item_object, position)
		WHERE name = $1 AND item_object->>'id' = $2)
	as int)
	WHERE name = $1;
	`, s.prefixedTable("services"), s.prefixedTable("services"))

	result, err := s.client.Exec(query, serviceName, id)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		return errors.New("unable to delete policy; an unknown error occurred")
	}
	return nil
}

func (s *Store) DeletePolicies(serviceName string) error {
	query := fmt.Sprintf("UPDATE %s SET policies = '[]'::jsonb WHERE name = $1", s.prefixedTable("services"))
	result, err := s.client.Exec(query, serviceName)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		return errors.New("unable to delete policies; an unknown error occurred")
	}
	return nil
}

func (s *Store) GetPolicy(serviceName string, id string) (*pms.Policy, error) {
	query := fmt.Sprintf(`
	SELECT item_object
	FROM %s, jsonb_array_elements(policies) WITH ORDINALITY arr(item_object, position)
	WHERE name = $1 AND item_object->>'id' = $2;
	`, s.prefixedTable("services"))

	row := s.client.QueryRow(query, serviceName, id)
	var rawPolicy []byte
	err := row.Scan(&rawPolicy)
	if err != nil {
		return nil, err
	}

	policy := &pms.Policy{}
	err = json.Unmarshal(rawPolicy, policy)
	return policy, err
}

func (s *Store) ListAllPolicies(serviceName string, filter string) ([]*pms.Policy, error) {
	query := fmt.Sprintf(`
	SELECT policies FROM %s
	WHERE name = $1
	`, s.prefixedTable("services"))

	row := s.client.QueryRow(query, serviceName)
	var rawPolicies []byte
	err := row.Scan(&rawPolicies)
	if err != nil {
		return nil, err
	}

	var policies []*pms.Policy
	err = json.Unmarshal(rawPolicies, &policies)
	return policies, err
}

func (s *Store) GetPolicyCount(serviceName string) (int64, error) {
	serviceTable := s.prefixedTable("services")
	var row *sql.Row
	if len(serviceName) != 0 {
		query := fmt.Sprintf(`SELECT COALESCE(jsonb_array_length(policies), 0) AS ct FROM %s WHERE name = $1`, serviceTable)
		row = s.client.QueryRow(query, serviceName)
	} else {
		query := fmt.Sprintf(`SELECT COALESCE(SUM(jsonb_array_length(policies)), 0) AS ct FROM %s`, serviceTable)
		row = s.client.QueryRow(query)
	}

	var count int64
	err := row.Scan(&count)
	return count, err
}

func (s *Store) CreateRolePolicy(serviceName string, policy *pms.RolePolicy) (*pms.RolePolicy, error) {
	policy.ID = uuid.NewRandom().String()
	jsonPolicy, err := json.Marshal(policy)
	if err != nil {
		return nil, err
	}

	query := fmt.Sprintf("UPDATE %s SET role_policies = role_policies || $1::jsonb WHERE name = $2", s.prefixedTable("services"))
	result, err := s.client.Exec(query, jsonPolicy, serviceName)
	if err != nil {
		return nil, err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}

	if rowCount == 0 {
		return nil, errors.New("unable to create role policy; an unknown error occurred")
	}
	return policy, nil
}

func (s *Store) DeleteRolePolicy(serviceName string, id string) error {
	query := fmt.Sprintf(`
	UPDATE %s SET role_policies = role_policies -
	CAST((
		SELECT position - 1
		FROM %s, jsonb_array_elements(role_policies) with ordinality arr(item_object, position)
		WHERE name = $1 AND item_object->>'id' = $2)
	as int)
	WHERE name = $1;
	`, s.prefixedTable("services"), s.prefixedTable("services"))

	result, err := s.client.Exec(query, serviceName, id)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		return errors.New("unable to delete role policy; an unknown error occurred")
	}
	return nil
}

func (s *Store) DeleteRolePolicies(serviceName string) error {
	query := fmt.Sprintf("UPDATE %s SET role_policies = '[]'::jsonb WHERE name = $1", s.prefixedTable("services"))
	result, err := s.client.Exec(query, serviceName)
	if err != nil {
		return err
	}

	rowCount, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowCount == 0 {
		return errors.New("unable to delete role policies; an unknown error occurred")
	}
	return nil
}

func (s *Store) GetRolePolicy(serviceName string, id string) (*pms.RolePolicy, error) {
	query := fmt.Sprintf(`
	SELECT item_object
	FROM %s, jsonb_array_elements(role_policies) WITH ORDINALITY arr(item_object, position)
	WHERE name = $1 AND item_object->>'id' = $2;
	`, s.prefixedTable("services"))

	row := s.client.QueryRow(query, serviceName, id)
	var rawPolicy []byte
	err := row.Scan(&rawPolicy)
	if err != nil {
		return nil, err
	}

	policy := &pms.RolePolicy{}
	err = json.Unmarshal(rawPolicy, policy)
	return policy, err
}

func (s *Store) ListAllRolePolicies(serviceName string, filter string) ([]*pms.RolePolicy, error) {
	query := fmt.Sprintf(`
	SELECT role_policies FROM %s
	WHERE name = $1
	`, s.prefixedTable("services"))

	row := s.client.QueryRow(query, serviceName)
	var rawPolicies []byte
	err := row.Scan(&rawPolicies)
	if err != nil {
		return nil, err
	}

	var policies []*pms.RolePolicy
	err = json.Unmarshal(rawPolicies, &policies)
	return policies, err
}

func (s *Store) GetRolePolicyCount(serviceName string) (int64, error) {
	serviceTable := s.prefixedTable("services")
	var row *sql.Row
	if len(serviceName) != 0 {
		query := fmt.Sprintf(`SELECT COALESCE(jsonb_array_length(role_policies), 0) AS ct FROM %s WHERE name = $1`, serviceTable)
		row = s.client.QueryRow(query, serviceName)
	} else {
		query := fmt.Sprintf(`SELECT COALESCE(SUM(jsonb_array_length(role_policies)), 0) AS ct FROM %s`, serviceTable)
		row = s.client.QueryRow(query)
	}
	var count int64
	err := row.Scan(&count)
	return count, err
}

func (s *Store) CreateFunction(function *pms.Function) (*pms.Function, error) {
	log.Info("create function")
	panic("not implemented") // TODO: Implement
}

func (s *Store) DeleteFunction(funcName string) error {
	log.Info("Delete function")
	panic("not implemented") // TODO: Implement
}

func (s *Store) DeleteFunctions() error {
	log.Info("Delete functions")
	panic("not implemented") // TODO: Implement
}

func (s *Store) GetFunction(funcName string) (*pms.Function, error) {
	log.Info("Get function")
	panic("not implemented") // TODO: Implement
}

func (s *Store) ListAllFunctions(filter string) ([]*pms.Function, error) {
	log.Info("list all functions")
	panic("not implemented") // TODO: Implement
}

func (s *Store) GetFunctionCount() (int64, error) {
	log.Info("Get function count")

	panic("not implemented") // TODO: Implement
}

func waitForNotification(l *pq.Listener) ChangeEvent {
	for {
		select {
		case n := <-l.Notify:
			fmt.Println("Received data from channel [", n.Channel, "] :")
			// Prepare notification payload for pretty print
			var event ChangeEvent
			err := json.Unmarshal([]byte(n.Extra), &event)

			if err != nil {
				fmt.Println("Error processing JSON: ", err)
				return ChangeEvent{}
			}
			return event
		case <-time.After(90 * time.Second):
			fmt.Println("Received no events for 90 seconds, checking connection")
			go func() {
				l.Ping()
			}()
			return ChangeEvent{}
		}
	}
}

func (s *Store) Watch() (pms.StorageChangeChannel, error) {
	log.Error("Enter Watch...")
	conninfo := "dbname=tinateams sslmode=disable"
	_, err := sql.Open("postgres", conninfo)
	if err != nil {
		panic(err)
	}

	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	listener := pq.NewListener(conninfo, 10*time.Second, time.Minute, reportProblem)
	err = listener.Listen("events")
	if err != nil {
		panic(err)
	}

	var storageChangeChan pms.StorageChangeChannel
	storageChangeChan = make(chan pms.StoreChangeEvent)

	go func() {
		defer func() {
			listener.Close()
			close(storageChangeChan)
		}()

		for {
			event := waitForNotification(listener)
			if event.Table == "speedle_services" {
				if event.Action == "UPDATE" {
					log.Info("===update service")
					id := time.Now().Unix()
					serviceDeleteEvent := pms.StoreChangeEvent{
						Type:    pms.SERVICE_DELETE,
						ID:      id,
						Content: []string{event.Service.Name},
					}
					log.Info("###serviceDeleteEvent:", serviceDeleteEvent)
					storageChangeChan <- serviceDeleteEvent

					id = time.Now().Unix()
					serviceAddEvent := pms.StoreChangeEvent{
						Type:    pms.SERVICE_ADD,
						ID:      id,
						Content: &event.Service,
					}
					log.Info("###serviceAddEvent", serviceAddEvent)
					storageChangeChan <- serviceAddEvent

				} else if event.Action == "DELETE" {
					log.Info("===delete service")
					id := time.Now().Unix()
					serviceDeleteEvent := pms.StoreChangeEvent{
						Type:    pms.SERVICE_DELETE,
						ID:      id,
						Content: []string{event.Service.Name},
					}
					log.Info("###serviceDeleteEvent:", serviceDeleteEvent)
					storageChangeChan <- serviceDeleteEvent

				} else if event.Action == "CREATE" {

					log.Info("===create service")
					id := time.Now().Unix()
					serviceAddEvent := pms.StoreChangeEvent{
						Type:    pms.SERVICE_ADD,
						ID:      id,
						Content: &event.Service,
					}
					log.Info("###serviceAddEvent", serviceAddEvent)
					storageChangeChan <- serviceAddEvent

				}

			}
		}
	}()

	return storageChangeChan, nil

	fmt.Println("Starting to monitor Postgres...")
	for {
		event := waitForNotification(listener)
		fmt.Println(event)
	}

	panic("Exited earlier")
}

func (s *Store) StopWatch() {
	log.Info("Stop watch!")
	panic("1not implemented") // TODO: Implement
}

func (s *Store) prefixedTable(name string) string {
	return fmt.Sprintf("%s%s", s.tablePrefix, name)
}
