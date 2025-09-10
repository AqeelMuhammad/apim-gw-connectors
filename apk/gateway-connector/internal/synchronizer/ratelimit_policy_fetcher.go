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

/*
 * Package "synchronizer" contains artifacts relate to fetching APIs and
 * API related updates from the control plane event-hub.
 * This file contains functions to retrieve APIs and API updates.
 */

package synchronizer

import (
	"time"

	k8sclient "github.com/wso2-extensions/apim-gw-connectors/apk/gateway-connector/internal/k8sClient"
	logger "github.com/wso2-extensions/apim-gw-connectors/apk/gateway-connector/internal/loggers"
	"github.com/wso2-extensions/apim-gw-connectors/common-agent/config"
	managementserver "github.com/wso2-extensions/apim-gw-connectors/common-agent/pkg/managementserver"
	sync "github.com/wso2-extensions/apim-gw-connectors/common-agent/pkg/synchronizer"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// !!! NOTE: This handles the normal ratelimit policies
// FetchRateLimitPoliciesOnEvent fetches the policies from the control plane on the start up and notification event updates
func FetchRateLimitPoliciesOnEvent(ratelimitName string, organization string, c client.Client) {
	// Read configurations and derive the eventHub details
	conf, errReadConfig := config.ReadConfigs()
	if errReadConfig != nil {
		// This has to be error. For debugging purpose info
		logger.LoggerSynchronizer.Errorf("Error reading configs: %v", errReadConfig)
	}
	logger.LoggerSynchronizer.Infof("Fetching rate limit policies on event for organization: %s", organization)
	logger.LoggerSynchronizer.Debugf("Rate Limit Name: %s | Organization: %s", ratelimitName, organization)
	rateLimitPolicies, errorMsg := sync.FetchRateLimitPoliciesOnEvent(ratelimitName, organization)
	if rateLimitPolicies != nil {
		if len(rateLimitPolicies) == 0 && errorMsg != "" {
			go retryRLPFetchData(conf, errorMsg, c)
		} else {
			for _, policy := range rateLimitPolicies {
				logger.LoggerSynchronizer.Debugf("Normal Ratelimit policy: %+v", policy)
				if policy.DefaultLimit.RequestCount.TimeUnit == "min" {
					policy.DefaultLimit.RequestCount.TimeUnit = "Minute"
				} else if policy.DefaultLimit.RequestCount.TimeUnit == "hour" {
					policy.DefaultLimit.RequestCount.TimeUnit = "Hour"
				} else if policy.DefaultLimit.RequestCount.TimeUnit == "day" {
					policy.DefaultLimit.RequestCount.TimeUnit = "Day"
				}
				// Update the exisitng rate limit policies with current policy
				k8sclient.UpdateRateLimitPolicyCR(policy, c)
				logger.LoggerSynchronizer.Debugf("RateLimit Policy updated: %v", policy)
			}
		}
	}
}

// FetchSubscriptionRateLimitPoliciesOnEvent fetches the policies from the control plane on the start up and notification event updates
func FetchSubscriptionRateLimitPoliciesOnEvent(ratelimitName string, organization string, c client.Client, cleanupDeletedPolicies bool) {
	// Read configurations and derive the eventHub details
	conf, errReadConfig := config.ReadConfigs()
	if errReadConfig != nil {
		// This has to be error. For debugging purpose info
		logger.LoggerSynchronizer.Errorf("Error reading configs: %v", errReadConfig)
	}
	logger.LoggerSynchronizer.Infof("Fetching rate limit policies on event for organization: %s", organization)
	logger.LoggerSynchronizer.Debugf("Rate Limit Name: %s | Organization: %s", ratelimitName, organization)
	rateLimitPolicies, errorMsg := sync.FetchSubscriptionRateLimitPoliciesOnEvent(ratelimitName, organization)
	if rateLimitPolicies != nil {
		if len(rateLimitPolicies) == 0 && errorMsg != "" {
			go retrySubscriptionRLPFetchData(conf, errorMsg, c)
		} else {
			if cleanupDeletedPolicies {
				logger.LoggerSynchronizer.Infof("Cleaning up deleted AI ratelimit policies")
				// !!!TODO: NEED TO ADD THE LOGIC
				// This logic is executed once at the startup time so no need to worry about the nested for loops for performance.
				deployedPolicyNames, retrieveErr := k8sclient.RetrieveSharedSubscriptionRateLimitPolicyFromK8s(organization, c)
				if retrieveErr == nil {
					for _, deployedPolicyName := range deployedPolicyNames {
						found := false
						for _, policy := range rateLimitPolicies {
							if policy.Name == deployedPolicyName {
								found = true
								break
							}
						}
						if !found {
							// Delete the rate limit policy rule
							logger.LoggerSynchronizer.Infof("Removing outdated rate limit policy: %s", deployedPolicyName)
							k8sclient.UnDeploySharedSubscriptionRateLimitPolicyCR(deployedPolicyName, organization, c, false)
						}
					}
				} else {
					logger.LoggerSynchronizer.Errorf("Error while fetching deployed rate limit policies for cleanup. Error: %+v", retrieveErr)
				}
				// // Fetch all AI RatelimitPolicies
				// airls, retrieveAllAIRLErr := k8sclient.RetrieveAllAIRatelimitPoliciesSFromK8s(ratelimitName, organization, c)
				// if retrieveAllAIRLErr == nil {
				// 	for _, airl := range airls {
				// 		if cpName, exists := airl.ObjectMeta.Labels["CPName"]; exists {
				// 			found := false
				// 			for _, policy := range rateLimitPolicies {
				// 				if policy.Name == cpName {
				// 					found = true
				// 					break
				// 				}
				// 			}
				// 			if !found {
				// 				// Delete the airatelimitpolicy
				// 				k8sclient.UnDeploySharedSubscriptionRateLimitPolicyCR(airl.Name, organization, c, true)
				// 			}
				// 		}
				// 	}
				// } else {
				// 	logger.LoggerSynchronizer.Errorf("Error while fetching airatelimitpolicies for cleaning up outdataed crs. Error: %+v", retrieveAllAIRLErr)
				// }

				// // Fetch all Normal RatelimitPolicies
				// rls, retrieveAllRLErr := k8sclient.RetrieveAllRatelimitPoliciesSFromK8s(ratelimitName, organization, c)
				// if retrieveAllRLErr == nil {
				// 	for _, rl := range rls {
				// 		if cpName, exists := rl.ObjectMeta.Labels["CPName"]; exists {
				// 			found := false
				// 			for _, policy := range rateLimitPolicies {
				// 				if policy.Name == cpName {
				// 					found = true
				// 					break
				// 				}
				// 			}
				// 			if !found {
				// 				// Delete the airatelimitpolicy
				// 				k8sclient.UnDeploySharedSubscriptionRateLimitPolicyCR(rl.Name, organization, c, false)
				// 			}
				// 		}
				// 	}
				// } else {
				// 	logger.LoggerSynchronizer.Errorf("Error while fetching ratelimitpolicies for cleaning up outdataed crs. Error: %+v", retrieveAllRLErr)
				// }
			}

			for _, policy := range rateLimitPolicies {
				if policy.QuotaType == "aiApiQuota" {
					logger.LoggerSynchronizer.Infof("AI Subscription RateLimit Policy detected...")
					logger.LoggerSynchronizer.Infof("AIAPIQuota data: %+v", policy.DefaultLimit.AiAPIQuota)
					if policy.DefaultLimit.AiAPIQuota != nil {
						// switch policy.DefaultLimit.AiAPIQuota.TimeUnit {
						// case "min":
						// 	policy.DefaultLimit.AiAPIQuota.TimeUnit = "Minute"
						// case "hours":
						// 	policy.DefaultLimit.AiAPIQuota.TimeUnit = "Hour"
						// case "days":
						// 	policy.DefaultLimit.AiAPIQuota.TimeUnit = "Day"
						// case "months":
						// 	policy.DefaultLimit.AiAPIQuota.TimeUnit = "Month"
						// default:
						// 	logger.LoggerSynchronizer.Errorf("Unsupported timeunit %s", policy.DefaultLimit.AiAPIQuota.TimeUnit)
						// 	continue
						// }
						// !!! Need to check what this logic is
						if policy.DefaultLimit.AiAPIQuota.PromptTokenCount == nil && policy.DefaultLimit.AiAPIQuota.TotalTokenCount != nil {
							policy.DefaultLimit.AiAPIQuota.PromptTokenCount = policy.DefaultLimit.AiAPIQuota.TotalTokenCount
						}
						if policy.DefaultLimit.AiAPIQuota.CompletionTokenCount == nil && policy.DefaultLimit.AiAPIQuota.TotalTokenCount != nil {
							policy.DefaultLimit.AiAPIQuota.CompletionTokenCount = policy.DefaultLimit.AiAPIQuota.TotalTokenCount
						}
						if policy.DefaultLimit.AiAPIQuota.TotalTokenCount == nil && policy.DefaultLimit.AiAPIQuota.PromptTokenCount != nil && policy.DefaultLimit.AiAPIQuota.CompletionTokenCount != nil {
							total := *policy.DefaultLimit.AiAPIQuota.PromptTokenCount + *policy.DefaultLimit.AiAPIQuota.CompletionTokenCount
							policy.DefaultLimit.AiAPIQuota.TotalTokenCount = &total
						}
						logger.LoggerSynchronizer.Infof("\n\nAI Subscription RateLimit Policy added from CP: %+v", policy)
						logger.LoggerSynchronizer.Infof("Policy Quota Type: %s", policy.QuotaType)
						logger.LoggerSynchronizer.Infof("AI -> PromptTokenCount: %d", *policy.DefaultLimit.AiAPIQuota.PromptTokenCount)
						logger.LoggerSynchronizer.Infof("AI -> CompletionTokenCount: %d", *policy.DefaultLimit.AiAPIQuota.CompletionTokenCount)
						logger.LoggerSynchronizer.Infof("AI -> TotalTokenCount: %d", *policy.DefaultLimit.AiAPIQuota.TotalTokenCount)
						managementserver.AddSubscriptionPolicy(policy)
						logger.LoggerSynchronizer.Infof("AI Subscription RateLimit Policy added to internal map")
						k8sclient.DeploySharedSubscriptionRateLimitPolicyCR(policy, c, true)
						logger.LoggerSynchronizer.Infof("AI RateLimit Policy added from CP Policy: %+v \n\n", policy)
					} else {
						logger.LoggerSynchronizer.Errorf("AIQuota type response recieved but no data found. %+v", policy.DefaultLimit)
					}
				} else {
					logger.LoggerSynchronizer.Infof("\n\nNormal Subscription RateLimit policy added from CP: %+v", policy)
					if policy.DefaultLimit.RequestCount.TimeUnit == "min" {
						policy.DefaultLimit.RequestCount.TimeUnit = "Minute"
					} else if policy.DefaultLimit.RequestCount.TimeUnit == "hours" {
						policy.DefaultLimit.RequestCount.TimeUnit = "Hour"
					} else if policy.DefaultLimit.RequestCount.TimeUnit == "days" {
						policy.DefaultLimit.RequestCount.TimeUnit = "Day"
					} else if policy.DefaultLimit.RequestCount.TimeUnit == "months" {
						policy.DefaultLimit.RequestCount.TimeUnit = "Month"
					}
					managementserver.AddSubscriptionPolicy(policy)
					logger.LoggerSynchronizer.Debug("Normal Subscription RateLimit Policy added to internal map")
					// Update the exisitng rate limit policies with current policy
					k8sclient.DeploySharedSubscriptionRateLimitPolicyCR(policy, c, false)
					logger.LoggerSynchronizer.Infof("Subscription RL Policy added: %+v \n\n", policy)
				}
			}
		}
	}
}

func retryRLPFetchData(conf *config.Config, errorMessage string, c client.Client) {
	logger.LoggerSynchronizer.Debugf("Time Duration for retrying: %v",
		conf.ControlPlane.RetryInterval*time.Second)
	time.Sleep(conf.ControlPlane.RetryInterval * time.Second)
	FetchRateLimitPoliciesOnEvent("", "", c)
	retryAttempt++
	if retryAttempt >= retryCount {
		logger.LoggerSynchronizer.Error(errorMessage)
		return
	}
}

func retrySubscriptionRLPFetchData(conf *config.Config, errorMessage string, c client.Client) {
	logger.LoggerSynchronizer.Debugf("Time Duration for retrying: %v",
		conf.ControlPlane.RetryInterval*time.Second)
	time.Sleep(conf.ControlPlane.RetryInterval * time.Second)
	FetchSubscriptionRateLimitPoliciesOnEvent("", "", c, false)
	retryAttempt++
	if retryAttempt >= retryCount {
		logger.LoggerSynchronizer.Error(errorMessage)
		return
	}
}
