/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package litellm

import "context"

// MockClient implements Client for testing.
type MockClient struct {
	MockModels        *MockModelService
	MockTeams         *MockTeamService
	MockUsers         *MockUserService
	MockKeys          *MockKeyService
	MockOrganizations *MockOrganizationService
	MockCustomers     *MockCustomerService
	MockHealth        *MockHealthService
}

// NewMockClient creates a new MockClient with default implementations.
func NewMockClient() *MockClient {
	return &MockClient{
		MockModels:        &MockModelService{},
		MockTeams:         &MockTeamService{},
		MockUsers:         &MockUserService{},
		MockKeys:          &MockKeyService{},
		MockOrganizations: &MockOrganizationService{},
		MockCustomers:     &MockCustomerService{},
		MockHealth:        &MockHealthService{},
	}
}

func (m *MockClient) Models() ModelService               { return m.MockModels }
func (m *MockClient) Teams() TeamService                 { return m.MockTeams }
func (m *MockClient) Users() UserService                 { return m.MockUsers }
func (m *MockClient) Keys() KeyService                   { return m.MockKeys }
func (m *MockClient) Organizations() OrganizationService { return m.MockOrganizations }
func (m *MockClient) Customers() CustomerService         { return m.MockCustomers }
func (m *MockClient) Health() HealthService              { return m.MockHealth }

// MockModelService records calls and returns configured responses.
type MockModelService struct {
	CreateFunc func(ctx context.Context, req ModelCreateRequest) (*ModelCreateResponse, error)
	UpdateFunc func(ctx context.Context, req ModelCreateRequest) error
	DeleteFunc func(ctx context.Context, modelID string) error
	GetFunc    func(ctx context.Context, modelID string) (*ModelInfoResponse, error)
	ListFunc   func(ctx context.Context) ([]ModelInfoResponse, error)
}

func (m *MockModelService) Create(ctx context.Context, req ModelCreateRequest) (*ModelCreateResponse, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &ModelCreateResponse{ModelID: "mock-model-id"}, nil
}

func (m *MockModelService) Update(ctx context.Context, req ModelCreateRequest) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, req)
	}
	return nil
}

func (m *MockModelService) Delete(ctx context.Context, modelID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, modelID)
	}
	return nil
}

func (m *MockModelService) Get(ctx context.Context, modelID string) (*ModelInfoResponse, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, modelID)
	}
	return &ModelInfoResponse{ModelID: modelID}, nil
}

func (m *MockModelService) List(ctx context.Context) ([]ModelInfoResponse, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

// MockTeamService records calls and returns configured responses.
type MockTeamService struct {
	CreateFunc         func(ctx context.Context, req TeamCreateRequest) (*TeamCreateResponse, error)
	UpdateFunc         func(ctx context.Context, req TeamUpdateRequest) error
	DeleteFunc         func(ctx context.Context, teamID string) error
	GetFunc            func(ctx context.Context, teamID string) (*TeamInfo, error)
	ListFunc           func(ctx context.Context) ([]TeamInfo, error)
	AddMemberFunc      func(ctx context.Context, teamID, email, role string) error
	RemoveMemberFunc   func(ctx context.Context, teamID, email string) error
	ListMembersFunc    func(ctx context.Context, teamID string) ([]TeamMemberInfo, error)
	SetCallbackFunc    func(ctx context.Context, teamID string, req TeamCallbackRequest) error
	DisableLoggingFunc func(ctx context.Context, teamID string) error
}

func (m *MockTeamService) Create(ctx context.Context, req TeamCreateRequest) (*TeamCreateResponse, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &TeamCreateResponse{TeamID: "mock-team-id", TeamAlias: req.TeamAlias}, nil
}

func (m *MockTeamService) Update(ctx context.Context, req TeamUpdateRequest) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, req)
	}
	return nil
}

func (m *MockTeamService) Delete(ctx context.Context, teamID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, teamID)
	}
	return nil
}

func (m *MockTeamService) Get(ctx context.Context, teamID string) (*TeamInfo, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, teamID)
	}
	return &TeamInfo{TeamID: teamID}, nil
}

func (m *MockTeamService) List(ctx context.Context) ([]TeamInfo, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockTeamService) AddMember(ctx context.Context, teamID, email, role string) error {
	if m.AddMemberFunc != nil {
		return m.AddMemberFunc(ctx, teamID, email, role)
	}
	return nil
}

func (m *MockTeamService) RemoveMember(ctx context.Context, teamID, email string) error {
	if m.RemoveMemberFunc != nil {
		return m.RemoveMemberFunc(ctx, teamID, email)
	}
	return nil
}

func (m *MockTeamService) ListMembers(ctx context.Context, teamID string) ([]TeamMemberInfo, error) {
	if m.ListMembersFunc != nil {
		return m.ListMembersFunc(ctx, teamID)
	}
	return nil, nil
}

func (m *MockTeamService) SetCallback(ctx context.Context, teamID string, req TeamCallbackRequest) error {
	if m.SetCallbackFunc != nil {
		return m.SetCallbackFunc(ctx, teamID, req)
	}
	return nil
}

func (m *MockTeamService) DisableLogging(ctx context.Context, teamID string) error {
	if m.DisableLoggingFunc != nil {
		return m.DisableLoggingFunc(ctx, teamID)
	}
	return nil
}

// MockUserService records calls and returns configured responses.
type MockUserService struct {
	CreateFunc func(ctx context.Context, req UserCreateRequest) (*UserCreateResponse, error)
	UpdateFunc func(ctx context.Context, req UserCreateRequest) error
	DeleteFunc func(ctx context.Context, userID string) error
	GetFunc    func(ctx context.Context, userID string) (*UserInfo, error)
	ListFunc   func(ctx context.Context) ([]UserInfo, error)
}

func (m *MockUserService) Create(ctx context.Context, req UserCreateRequest) (*UserCreateResponse, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &UserCreateResponse{UserID: req.UserID}, nil
}

func (m *MockUserService) Update(ctx context.Context, req UserCreateRequest) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, req)
	}
	return nil
}

func (m *MockUserService) Delete(ctx context.Context, userID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, userID)
	}
	return nil
}

func (m *MockUserService) Get(ctx context.Context, userID string) (*UserInfo, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, userID)
	}
	return &UserInfo{UserID: userID}, nil
}

func (m *MockUserService) List(ctx context.Context) ([]UserInfo, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

// MockKeyService records calls and returns configured responses.
type MockKeyService struct {
	GenerateFunc func(ctx context.Context, req KeyGenerateRequest) (*KeyGenerateResponse, error)
	UpdateFunc   func(ctx context.Context, req KeyUpdateRequest) error
	DeleteFunc   func(ctx context.Context, token string) error
	GetFunc      func(ctx context.Context, token string) (*KeyInfo, error)
	ListFunc     func(ctx context.Context) ([]KeyInfo, error)
}

func (m *MockKeyService) Generate(ctx context.Context, req KeyGenerateRequest) (*KeyGenerateResponse, error) {
	if m.GenerateFunc != nil {
		return m.GenerateFunc(ctx, req)
	}
	return &KeyGenerateResponse{Key: "sk-mock-key", Token: "mock-token-hash"}, nil
}

func (m *MockKeyService) Update(ctx context.Context, req KeyUpdateRequest) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, req)
	}
	return nil
}

func (m *MockKeyService) Delete(ctx context.Context, token string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, token)
	}
	return nil
}

func (m *MockKeyService) Get(ctx context.Context, token string) (*KeyInfo, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, token)
	}
	return &KeyInfo{Token: token, IsActive: true}, nil
}

func (m *MockKeyService) List(ctx context.Context) ([]KeyInfo, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

// MockOrganizationService records calls and returns configured responses.
type MockOrganizationService struct {
	CreateFunc       func(ctx context.Context, req OrganizationCreateRequest) (*OrganizationCreateResponse, error)
	UpdateFunc       func(ctx context.Context, req OrganizationUpdateRequest) error
	DeleteFunc       func(ctx context.Context, organizationID string) error
	GetFunc          func(ctx context.Context, organizationID string) (*OrganizationInfo, error)
	ListFunc         func(ctx context.Context) ([]OrganizationInfo, error)
	AddMemberFunc    func(ctx context.Context, req OrgMemberAddRequest) error
	DeleteMemberFunc func(ctx context.Context, organizationID, userID string) error
}

func (m *MockOrganizationService) Create(ctx context.Context, req OrganizationCreateRequest) (*OrganizationCreateResponse, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &OrganizationCreateResponse{OrganizationID: "mock-org-id", OrganizationAlias: req.OrganizationAlias}, nil
}

func (m *MockOrganizationService) Update(ctx context.Context, req OrganizationUpdateRequest) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, req)
	}
	return nil
}

func (m *MockOrganizationService) Delete(ctx context.Context, organizationID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, organizationID)
	}
	return nil
}

func (m *MockOrganizationService) Get(ctx context.Context, organizationID string) (*OrganizationInfo, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, organizationID)
	}
	return &OrganizationInfo{OrganizationID: organizationID}, nil
}

func (m *MockOrganizationService) List(ctx context.Context) ([]OrganizationInfo, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

func (m *MockOrganizationService) AddMember(ctx context.Context, req OrgMemberAddRequest) error {
	if m.AddMemberFunc != nil {
		return m.AddMemberFunc(ctx, req)
	}
	return nil
}

func (m *MockOrganizationService) DeleteMember(ctx context.Context, organizationID, userID string) error {
	if m.DeleteMemberFunc != nil {
		return m.DeleteMemberFunc(ctx, organizationID, userID)
	}
	return nil
}

// MockCustomerService records calls and returns configured responses.
type MockCustomerService struct {
	CreateFunc func(ctx context.Context, req CustomerCreateRequest) (*CustomerInfo, error)
	UpdateFunc func(ctx context.Context, req CustomerUpdateRequest) (*CustomerInfo, error)
	DeleteFunc func(ctx context.Context, customerID string) error
	GetFunc    func(ctx context.Context, customerID string) (*CustomerInfo, error)
	ListFunc   func(ctx context.Context) ([]CustomerInfo, error)
}

func (m *MockCustomerService) Create(ctx context.Context, req CustomerCreateRequest) (*CustomerInfo, error) {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, req)
	}
	return &CustomerInfo{UserID: req.UserID, Alias: req.Alias, MaxBudget: req.MaxBudget}, nil
}

func (m *MockCustomerService) Update(ctx context.Context, req CustomerUpdateRequest) (*CustomerInfo, error) {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, req)
	}
	return &CustomerInfo{UserID: req.UserID, Alias: req.Alias, MaxBudget: req.MaxBudget}, nil
}

func (m *MockCustomerService) Delete(ctx context.Context, customerID string) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, customerID)
	}
	return nil
}

func (m *MockCustomerService) Get(ctx context.Context, customerID string) (*CustomerInfo, error) {
	if m.GetFunc != nil {
		return m.GetFunc(ctx, customerID)
	}
	return &CustomerInfo{UserID: customerID}, nil
}

func (m *MockCustomerService) List(ctx context.Context) ([]CustomerInfo, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx)
	}
	return nil, nil
}

// MockHealthService records calls and returns configured responses.
type MockHealthService struct {
	CheckLivenessFunc  func(ctx context.Context) error
	CheckReadinessFunc func(ctx context.Context) error
	ReadinessFunc      func(ctx context.Context) (*ReadinessResponse, error)
	CheckFunc          func(ctx context.Context) (*HealthCheckResponse, error)
}

func (m *MockHealthService) CheckLiveness(ctx context.Context) error {
	if m.CheckLivenessFunc != nil {
		return m.CheckLivenessFunc(ctx)
	}
	return nil
}

func (m *MockHealthService) CheckReadiness(ctx context.Context) error {
	if m.CheckReadinessFunc != nil {
		return m.CheckReadinessFunc(ctx)
	}
	return nil
}

func (m *MockHealthService) Readiness(ctx context.Context) (*ReadinessResponse, error) {
	if m.ReadinessFunc != nil {
		return m.ReadinessFunc(ctx)
	}
	return &ReadinessResponse{Status: "connected", RedisConnected: true}, nil
}

func (m *MockHealthService) Check(ctx context.Context) (*HealthCheckResponse, error) {
	if m.CheckFunc != nil {
		return m.CheckFunc(ctx)
	}
	return &HealthCheckResponse{}, nil
}
