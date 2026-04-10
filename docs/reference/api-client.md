# LiteLLM API Client

The operator communicates with LiteLLM through a REST API client defined as Go interfaces. This enables testing with mock implementations.

## Client Interface

```go
type Client interface {
    Models()        ModelService
    Teams()         TeamService
    Users()         UserService
    Keys()          KeyService
    Organizations() OrganizationService
    Health()        HealthService
}
```

A `ClientFactory` function creates clients for each LiteLLM instance:

```go
type ClientFactory func(endpoint, masterKey string) Client
```

## Service Interfaces

### ModelService

| Method | HTTP | Endpoint | Description |
| --- | --- | --- | --- |
| `Create` | POST | `/model/new` | Register a new model |
| `Update` | POST | `/model/update` | Update model configuration |
| `Delete` | POST | `/model/delete` | Remove a model |
| `Get` | GET | `/model/info?litellm_model_id=` | Get model info |
| `List` | GET | `/model/info` | List all models |

### TeamService

| Method | HTTP | Endpoint | Description |
| --- | --- | --- | --- |
| `Create` | POST | `/team/new` | Create a team |
| `Update` | POST | `/team/update` | Update team properties |
| `Delete` | POST | `/team/delete` | Delete a team |
| `Get` | GET | `/team/info?team_id=` | Get team info |
| `List` | GET | `/team/list` | List all teams |
| `AddMember` | POST | `/team/member_add` | Add a team member |
| `RemoveMember` | POST | `/team/member_delete` | Remove a team member |
| `ListMembers` | GET | `/team/info?team_id=` | List team members |

### UserService

| Method | HTTP | Endpoint | Description |
| --- | --- | --- | --- |
| `Create` | POST | `/user/new` | Create a user |
| `Update` | POST | `/user/update` | Update user properties |
| `Delete` | POST | `/user/delete` | Delete a user |
| `Get` | GET | `/user/info?user_id=` | Get user info |
| `List` | GET | `/user/list` | List all users |

### KeyService

| Method | HTTP | Endpoint | Description |
| --- | --- | --- | --- |
| `Generate` | POST | `/key/generate` | Generate a virtual key |
| `Update` | POST | `/key/update` | Update key properties |
| `Delete` | POST | `/key/delete` | Revoke a key |
| `Get` | GET | `/key/info?key=` | Get key info |
| `List` | GET | `/key/list` | List all keys |

### OrganizationService

| Method | HTTP | Endpoint | Description |
| --- | --- | --- | --- |
| `Create` | POST | `/organization/new` | Create an organization |
| `Update` | POST | `/organization/update` | Update organization properties |
| `Delete` | POST | `/organization/delete` | Delete an organization |
| `Get` | GET | `/organization/info?organization_id=` | Get organization info |
| `List` | GET | `/organization/list` | List all organizations |
| `AddMember` | POST | `/organization/member_add` | Add an organization member |
| `DeleteMember` | POST | `/organization/member_delete` | Remove an organization member |

### HealthService

| Method | HTTP | Endpoint | Description |
| --- | --- | --- | --- |
| `CheckLiveness` | GET | `/health/liveliness` | Liveness check |
| `CheckReadiness` | GET | `/health/readiness` | Readiness check |

## Authentication

All API requests include the master key as a Bearer token:

```
Authorization: Bearer <master_key>
```

## Error Handling

API errors are wrapped in an `APIError` type:

```go
type APIError struct {
    StatusCode int
    Message    string
    Path       string
}
```

Helper methods:
- `IsNotFound()` — returns true for 404 responses
- `IsTransient()` — returns true for 5xx responses (triggers requeue)

## Mock Client

For testing, use `litellm.NewMockClient()`:

```go
mock := litellm.NewMockClient()

// Configure custom behavior
mock.MockModels.CreateFunc = func(ctx context.Context, req ModelCreateRequest) (*ModelCreateResponse, error) {
    return &ModelCreateResponse{ModelID: "custom-id"}, nil
}

// Use in controller tests
reconciler := &LiteLLMModelReconciler{
    LiteLLMClientFactory: func(endpoint, masterKey string) litellm.Client {
        return mock
    },
}
```
