/*
 *  Copyright (c) 2024, WSO2 LLC. (http://www.wso2.org) All Rights Reserved.
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

package types

// Subscription for struct subscription
type Subscription struct {
	SubscriptionID          int32  `json:"subscriptionId"`
	SubscriptionUUID        string `json:"subscriptionUUID"`
	PolicyID                string `json:"policyId"`
	APIID                   int32  `json:"apiId"`
	APIUUID                 string `json:"apiUUID"`
	AppID                   int32  `json:"appId" json:"applicationId"`
	ApplicationUUID         string `json:"applicationUUID"`
	SubscriptionState       string `json:"subscriptionState"`
	TenantID                int32  `json:"tenanId,omitempty"`
	APIOrganization         string `json:"apiOrganization,omitempty"`
	ApplicationOrganization string `json:"applicationOrganization,omitempty"`
	TimeStamp               int64  `json:"timeStamp,omitempty"`
	APIName                 string `json:"apiName"`
	APIVersion              string `json:"apiVersion"`
}

// SubscriptionList for struct list of applications
type SubscriptionList struct {
	List []Subscription `json:"list"`
}

// API for struct Api
type API struct {
	APIID            int    `json:"apiId"`
	UUID             string `json:"uuid"`
	Provider         string `json:"provider" json:"apiProvider"`
	Name             string `json:"name" json:"apiName"`
	Version          string `json:"version" json:"apiVersion"`
	Context          string `json:"context" json:"apiContext"`
	Policy           string `json:"policy"`
	APIType          string `json:"apiType"`
	IsDefaultVersion bool   `json:"isDefaultVersion"`
	APIStatus        string `json:"status"`
	TenantID         int32  `json:"tenanId,omitempty"`
	TenantDomain     string `json:"tenanDomain,omitempty"`
	TimeStamp        int64  `json:"timeStamp,omitempty"`
}

// APIList for struct ApiList
type APIList struct {
	List []API `json:"list"`
}

// SubscriptionPolicy for struct list of SubscriptionPolicy
type SubscriptionPolicy struct {
	TenantID             int32        `json:"tenantId"`
	TenantDomain         string       `json:"tenantDomain,omitempty"`
	Name                 string       `json:"name"`
	QuotaType            string       `json:"quotaType"`
	GraphQLMaxComplexity int32        `json:"graphQLMaxComplexity"`
	GraphQLMaxDepth      int32        `json:"graphQLMaxDepth"`
	RateLimitCount       int32        `json:"rateLimitCount"`
	RateLimitTimeUnit    string       `json:"rateLimitTimeUnit"`
	StopOnQuotaReach     bool         `json:"stopOnQuotaReach"`
	DefaultLimit         DefaultLimit `json:"defaultLimit"`
	TimeStamp            int64        `json:"timeStamp,omitempty"`
}

// SubscriptionPolicyList for struct list of SubscriptionPolicy
type SubscriptionPolicyList struct {
	Count int                  `json:"count"`
	List  []SubscriptionPolicy `json:"list"`
}

// AIProviderList for struct list of AIProvider
type AIProviderList struct {
	AIProviders []AIProvider `json:"llmProviders"`
}

// AIProvider for struct AIProvider
type AIProvider struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	APIVersion     string `json:"apiVersion"`
	Organization   string `json:"organization"`
	Configurations string `json:"configurations"` // Initially treat as string
}

// Config for struct Metadata
type Config struct {
	Metadata   []Fields `json:"metadata"`
	AuthHeader string   `json:"authHeader"`
}

// Fields for struct Fields
type Fields struct {
	AttributeName       string `json:"attributeName"`
	InputSource         string `json:"inputSource"`
	AttributeIdentifier string `json:"attributeIdentifier"`
}

// RateLimitPolicyList for struct list of RateLimitPolicy
type RateLimitPolicyList struct {
	Count int               `json:"count"`
	List  []RateLimitPolicy `json:"list"`
}

// RateLimitPolicy for struct RateLimitPolicy Info events
type RateLimitPolicy struct {
	TenantDomain    string           `json:"tenantDomain"`
	Name            string           `json:"name"`
	QuotaType       string           `json:"quotaType"`
	ConditionGroups []ConditionGroup `json:"conditionGroups"`
	ApplicableLevel string           `json:"applicableLevel"`
	DefaultLimit    DefaultLimit     `json:"defaultLimit"`
}

// ConditionGroup represents the condition group within the response.
type ConditionGroup struct {
	PolicyID         int         `json:"policyId"`
	QuotaType        interface{} `json:"quotaType"`
	ConditionGroupID int         `json:"conditionGroupId"`
	Condition        []string    `json:"condition"`
	DefaultLimit     interface{} `json:"defaultLimit"`
}

// DefaultLimit represents the default limit within the response.
type DefaultLimit struct {
	AiAPIQuota   *AiAPIQuota `json:"aiApiQuota"`
	QuotaType    string      `json:"quotaType"`
	RequestCount struct {
		TimeUnit     string `json:"timeUnit"`
		UnitTime     int    `json:"unitTime"`
		RequestCount int    `json:"requestCount"`
	} `json:"requestCount"`
	Bandwidth  interface{} `json:"bandwidth"`
	EventCount interface{} `json:"eventCount"`
}

// AiAPIQuota contains the AI ratelimit configurations
type AiAPIQuota struct {
	CompletionTokenCount *int   `json:"completionTokenCount"`
	PromptTokenCount     *int   `json:"promptTokenCount"`
	RequestCount         *int   `json:"requestCount"`
	TimeUnit             string `json:"timeUnit"`
	TotalTokenCount      *int   `json:"totalTokenCount"`
	UnitTime             int    `json:"unitTime"`
}

// Scope for struct Scope
type Scope struct {
	Name            string `json:"name"`
	DisplayName     string `json:"displayName"`
	ApplicationName string `json:"description"`
}

// ScopeList for struct list of Scope
type ScopeList struct {
	List []Scope `json:"list"`
}

// KeyManagerList for struct list of KeyManager
type KeyManagerList struct {
	KeyManagers []KeyManager `json:"KeyManager"`
}

// KeyManager for struct
type KeyManager struct {
	UUID          string                 `json:"uuid"`
	Name          string                 `json:"name"`
	Type          string                 `json:"type"`
	Enabled       bool                   `json:"enabled"`
	Organization  string                 `json:"organization,omitempty"`
	TokenType     string                 `json:"tokenType"`
	Configuration map[string]interface{} `json:"additionalProperties"`
}

// ResolvedKeyManager for struct
type ResolvedKeyManager struct {
	UUID             string           `json:"uuid"`
	Name             string           `json:"name"`
	Type             string           `json:"type"`
	Enabled          bool             `json:"enabled"`
	Organization     string           `json:"organization,omitempty"`
	TokenType        string           `json:"tokenType"`
	KeyManagerConfig KeyManagerConfig `json:"configuration"`
}

// KeyManagerConfig for struct Configuration map[string]interface{} `json:"value"`
type KeyManagerConfig struct {
	TokenFormatString          string   `json:"token_format_string"`
	ServerURL                  string   `json:"ServerURL"`
	ValidationEnable           bool     `json:"validation_enable"`
	ClaimMappings              []Claim  `json:"claim_mappings"`
	GrantTypes                 []string `json:"grant_types"`
	EncryptPersistedTokens     bool     `json:"OAuthConfigurations.EncryptPersistedTokens"`
	EnableOauthAppCreation     bool     `json:"enable_oauth_app_creation"`
	ValidityPeriod             string   `json:"VALIDITY_PERIOD"`
	EnableTokenGeneration      bool     `json:"enable_token_generation"`
	Issuer                     string   `json:"issuer"`
	EnableMapOauthConsumerApps bool     `json:"enable_map_oauth_consumer_apps"`
	EnableTokenHash            bool     `json:"enable_token_hash"`
	SelfValidateJwt            bool     `json:"self_validate_jwt"`
	RevokeEndpoint             string   `json:"revoke_endpoint"`
	EnableTokenEncryption      bool     `json:"enable_token_encryption"`
	RevokeURL                  string   `json:"RevokeURL"`
	TokenURL                   string   `json:"TokenURL,token_endpoint"`
	CertificateType            string   `json:"certificate_type"`
	CertificateValue           string   `json:"certificate_value"`
	ConsumerKeyClaim           string   `json:"consumer_key_claim"`
	ScopesClaim                string   `json:"scopes_claim"`
}

// Claim for struct
type Claim struct {
	RemoteClaim string `json:"remoteClaim"`
	LocalClaim  string `json:"localClaim"`
}
