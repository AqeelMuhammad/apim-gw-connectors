/*
 *  Copyright (c) 2025, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
 *
 *  Licensed under the Apache License, Version 2.0 (the "License");
 *  you may not use this file except in compliance with the License.
 *  You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 *  Unless required by applicable law or agreed to in writing, software
 *  distributed under the License is distributed on an "AS IS" BASIS,
 *  WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 *  See the License for the specific language governing permissions and
 *  limitations under the License.
 *
 */

package transformer

import "github.com/wso2-extensions/apim-gw-connectors/common-agent/pkg/eventhub/types"

// SecretInfo holds the info related to the created secret upon enabling the endpoint security options like basic auth
type SecretInfo struct {
	SecretName     string `yaml:"secretName,omitempty"`
	UsernameKey    string `yaml:"userNameKey,omitempty"`
	PasswordKey    string `yaml:"passwordKey,omitempty"`
	In             string `yaml:"in,omitempty"`
	APIKeyNameKey  string `yaml:"apiKeyNameKey,omitempty"`
	APIKeyValueKey string `yaml:"apiKeyValueKey,omitempty"`
}

// EndpointSecurity comtains the information related to endpoint security configurations enabled by a user for a given API
type EndpointSecurity struct {
	Enabled      bool       `yaml:"enabled,omitempty"`
	SecurityType SecretInfo `yaml:"securityType,omitempty"`
}

// EndpointCertificate struct stores the the alias and the name for a particular endpoint security configuration
type EndpointCertificate struct {
	Name string `yaml:"secretName"`
	Key  string `yaml:"secretKey"`
}

// EndpointConfiguration stores the data related to endpoints and their related
type EndpointConfiguration struct {
	Endpoint       string              `yaml:"endpoint,omitempty"`
	EndCertificate EndpointCertificate `yaml:"certificate,omitempty"`
	EndSecurity    EndpointSecurity    `yaml:"endpointSecurity,omitempty"`
	AIRatelimit    AIRatelimit         `yaml:"aiRatelimit,omitempty"`
}

// AIRatelimit defines the configuration for AI rate limiting,
// including whether rate limiting is enabled and the settings
// for token and request-based limits.
type AIRatelimit struct {
	Enabled bool        `yaml:"enabled"`
	Token   TokenAIRL   `yaml:"token"`
	Request RequestAIRL `yaml:"request"`
}

// TokenAIRL defines the configuration for Token AI rate limit settings.
type TokenAIRL struct {
	PromptLimit     int    `yaml:"promptLimit"`
	CompletionLimit int    `yaml:"completionLimit"`
	TotalLimit      int    `yaml:"totalLimit"`
	Unit            string `yaml:"unit"` // Time unit (Minute, Hour, Day)
}

// RequestAIRL defines the configuration for Request AI rate limit settings.
type RequestAIRL struct {
	RequestLimit int    `yaml:"requestLimit"`
	Unit         string `yaml:"unit"` // Time unit (Minute, Hour, Day)
}

// AdditionalProperty stores the custom properties set by the user for a particular API
type AdditionalProperty struct {
	Name  string `yaml:"name"`
	Value string `yaml:"value"`
}

// Certificate struct stores the the alias and the name for a particular mTLS configuration
type Certificate struct {
	Name string `json:"name"`
	Key  string `json:"key"`
}

// AuthConfiguration represents the security configurations made for the API security
type AuthConfiguration struct {
	Required          string        `yaml:"required,omitempty"`
	AuthType          string        `yaml:"authType,omitempty"`
	HeaderName        string        `yaml:"headerName,omitempty"`
	SendTokenUpStream bool          `yaml:"sendTokenToUpstream,omitempty"`
	Enabled           bool          `yaml:"enabled"`
	QueryParamName    string        `yaml:"queryParamName,omitempty"`
	HeaderEnabled     bool          `yaml:"headerEnable,omitempty"`
	QueryParamEnable  bool          `yaml:"queryParamEnable,omitempty"`
	Certificates      []Certificate `yaml:"certificates,omitempty"`
	Audience          []string      `yaml:"audience,omitempty"`
}

// Endpoint represents an API endpoint.
// type Endpoint struct {
// 	Endpoint string `yaml:"endpoint,omitempty"`
// }

// EndpointConfigurations holds production and sandbox endpoints.
type EndpointConfigurations struct {
	Production *[]EndpointConfiguration `yaml:"production,omitempty"`
	Sandbox    *[]EndpointConfiguration `yaml:"sandbox,omitempty"`
}

// OperationPolicies organizes request and response policies for an API operation.
type OperationPolicies struct {
	Request  []OperationPolicy `yaml:"request,omitempty"`
	Response []OperationPolicy `yaml:"response,omitempty"`
}

// Operation represents an API operation with target, verb, scopes, security, and associated policies.
type Operation struct {
	Target            string             `yaml:"target,omitempty"`
	Verb              string             `yaml:"verb,omitempty"`
	Scopes            []string           `yaml:"scopes"`
	Secured           bool               `yaml:"secured"`
	OperationPolicies *OperationPolicies `yaml:"operationPolicies,omitempty"`
	RateLimit         *RateLimit         `yaml:"rateLimit,omitempty"`
}

// RateLimit is a placeholder for future rate-limiting configuration.
type RateLimit struct {
	RequestsPerUnit int    `yaml:"requestsPerUnit,omitempty"`
	Unit            string `yaml:"unit,omitempty"`
}

// VHost defines virtual hosts for production and sandbox environments.
type VHost struct {
	Production []string `yaml:"production,omitempty"`
	Sandbox    []string `yaml:"sandbox,omitempty"`
}

// AIProvider represents the AI provider configuration.
type AIProvider struct {
	Name       string `json:"name,omitempty" yaml:"name,omitempty"`
	APIVersion string `json:"apiVersion,omitempty" yaml:"apiVersion,omitempty"`
}

// // Claim represents a claim mapping between local and remote claims.
// type Claim struct {
//     LocalClaim  string `json:"localClaim" yaml:"localClaim"`
//     RemoteClaim string `json:"remoteClaim" yaml:"remoteClaim"`
// }

// K8sBackendConfig represents the configuration for the k8s backend
type K8sBackendConfig struct {
	Name      string `json:"name" yaml:"name"`
	Port      int    `json:"port" yaml:"port"`
	Namespace string `json:"namespace" yaml:"namespace"`
}

// KeyManager represents the properties of a particular key manager.
type KeyManager struct {
	Name         string            `json:"name" yaml:"name"`
	Issuer       string            `json:"issuer" yaml:"issuer"`
	JWKSEndpoint string            `json:"JWKSEndpoint" yaml:"JWKSEndpoint"`
	ClaimMapping []types.Claim     `json:"claimMappings" yaml:"claimMappings"`
	K8sBackend   *K8sBackendConfig `json:"k8sBackend" yaml:"k8sBackend"`
}

// API represents an main API type definition
type API struct {
	Name                   string                  `yaml:"name,omitempty"`
	ID                     string                  `yaml:"id,omitempty"`
	Version                string                  `yaml:"version,omitempty"`
	Context                string                  `yaml:"basePath,omitempty"`
	Type                   string                  `yaml:"type,omitempty"`
	DefaultVersion         bool                    `yaml:"defaultVersion"`
	Environment            string                  `yaml:"environment,omitempty"`
	DefinitionPath         string                  `yaml:"definitionPath,omitempty"`
	EndpointConfigurations *EndpointConfigurations `yaml:"endpointConfigurations,omitempty"`
	Operations             *[]Operation            `yaml:"operations,omitempty"`
	Authentication         *[]AuthConfiguration    `yaml:"authentication,omitempty"`
	CorsConfig             *CORSConfiguration      `yaml:"corsConfiguration,omitempty"`
	AdditionalProperties   *[]AdditionalProperty   `yaml:"additionalProperties,omitempty"`
	SubscriptionValidation bool                    `yaml:"subscriptionValidation,omitempty"`
	RateLimit              *RateLimit              `yaml:"rateLimit,omitempty"`
	APIPolicies            *OperationPolicies      `yaml:"apiPolicies,omitempty"`
	AIProvider             *AIProvider             `yaml:"aiProvider,omitempty"`
	KeyManagers            *[]KeyManager           `json:"keyManagers,omitempty" yaml:"keyManagers,omitempty"`
}
