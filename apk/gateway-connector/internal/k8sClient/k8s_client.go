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

// Package k8sclient contains the common implementation methods to invoke k8s APIs in the agent
package k8sclient

import (
	"context"
	"errors"
	"fmt"
	"strings"

	gatewayv1alpha1 "github.com/envoyproxy/gateway/api/v1alpha1"
	"github.com/wso2-extensions/apim-gw-connectors/apk/gateway-connector/internal/constants"
	"github.com/wso2-extensions/apim-gw-connectors/apk/gateway-connector/internal/loggers"
	"github.com/wso2-extensions/apim-gw-connectors/apk/gateway-connector/internal/logging"
	"github.com/wso2-extensions/apim-gw-connectors/apk/gateway-connector/internal/utils"
	"github.com/wso2-extensions/apim-gw-connectors/common-agent/config"
	"github.com/wso2-extensions/apim-gw-connectors/common-agent/pkg/cache"
	eventhubTypes "github.com/wso2-extensions/apim-gw-connectors/common-agent/pkg/eventhub/types"
	dpv2alpha1 "github.com/wso2/apk/common-go-libs/apis/dp/v2alpha1"
	corev1 "k8s.io/api/core/v1"
	k8error "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/client"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
	v1 "sigs.k8s.io/gateway-api/apis/v1"
	gwapiv1a2 "sigs.k8s.io/gateway-api/apis/v1alpha2"
	gwapiv1a3 "sigs.k8s.io/gateway-api/apis/v1alpha3"
)

// !!! ======== NEW ========

const (
	// Shared BackendTrafficPolicy name for all subscription rate limit policies
	SharedRateLimitPolicyName = "kgw-shared-subscription-rl-policy"
)

// UndeployRouteMetadataCRs removes all RouteMetadata Custom Resource from the Kubernetes cluster based on API ID label.
func UndeployRouteMetadataCRs(apiID string, k8sClient client.Client) {
	conf, errReadConfig := config.ReadConfigs()
	if errReadConfig != nil {
		loggers.LoggerK8sClient.Errorf("Error reading configurations: %v", errReadConfig)
	}
	routeMetadataList := &dpv2alpha1.RouteMetadataList{}
	err := k8sClient.List(context.Background(), routeMetadataList, &client.ListOptions{Namespace: conf.DataPlane.Namespace, LabelSelector: labels.SelectorFromSet(map[string]string{"apiUUID": apiID})})
	// Retrieve all API CRs from the Kubernetes cluster
	if err != nil {
		loggers.LoggerK8sClient.Errorf("Unable to list RouteMetadata CRs: %v", err)
	}
	for _, routemeta := range routeMetadataList.Items {
		if err := UndeployK8sRouteMetadataCRs(k8sClient, routemeta); err != nil {
			loggers.LoggerK8sClient.Errorf("Unable to delete RouteMetadata CR: %v", err)
		}
		loggers.LoggerK8sClient.Infof("Deleted RouteMetadata CR: %s", routemeta.Name)
	}
}

// UndeployK8sRouteMetadataCRs removes specific RouteMetadata CR from the Kubernetes cluster based on RouteMetadata name.
func UndeployK8sRouteMetadataCRs(k8sClient client.Client, k8sRouteMetadata dpv2alpha1.RouteMetadata) error {
	err := k8sClient.Delete(context.Background(), &k8sRouteMetadata, &client.DeleteOptions{})
	if err != nil {
		loggers.LoggerK8sClient.Errorf("Unable to delete RouteMetadata CR: %v", err)
		return err
	}
	loggers.LoggerK8sClient.Infof("Deleted RouteMetadata CR: %s", k8sRouteMetadata.Name)
	return nil
}

// DeployRouteMetadataCR applies the given RouteMetadata struct to the Kubernetes cluster.
func DeployRouteMetadataCR(routeMetadata *dpv2alpha1.RouteMetadata, k8sClient client.Client) (types.UID, error) {
	crRouteMetadata := &dpv2alpha1.RouteMetadata{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: routeMetadata.ObjectMeta.Namespace, Name: routeMetadata.Name}, crRouteMetadata); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get RouteMetadata CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), routeMetadata); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create RouteMetadata CR: " + err.Error())
			return "", err
		} else {
			loggers.LoggerK8sClient.Info("RouteMetadata CR created: " + routeMetadata.Name)
			return routeMetadata.ObjectMeta.UID, nil
		}
	} else {
		crRouteMetadata.Spec = routeMetadata.Spec
		if err := k8sClient.Update(context.Background(), crRouteMetadata); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update RouteMetadata CR: " + err.Error())
			return "", err
		} else {
			loggers.LoggerK8sClient.Info("RouteMetadata CR updated: " + routeMetadata.Name)
			return crRouteMetadata.ObjectMeta.UID, nil
		}
	}
}

// DeployConfigMapCR applies the given ConfigMap struct to the Kubernetes cluster.
func DeployConfigMapCR(configMap *corev1.ConfigMap, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		configMap.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crConfigMap := &corev1.ConfigMap{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: configMap.ObjectMeta.Namespace, Name: configMap.Name}, crConfigMap); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get ConfigMap CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), configMap); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create ConfigMap CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("ConfigMap CR created: " + configMap.Name)
		}
	} else {
		crConfigMap.Data = configMap.Data
		if err := k8sClient.Update(context.Background(), crConfigMap); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update ConfigMap CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("ConfigMap CR updated: " + configMap.Name)
		}
	}
}

// DeployHTTPRouteCR applies the given HttpRoute struct to the Kubernetes cluster.
func DeployHTTPRouteFilterCR(httpRouteFilter *gatewayv1alpha1.HTTPRouteFilter, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		httpRouteFilter.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crHTTPRouteFilter := &gatewayv1alpha1.HTTPRouteFilter{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: httpRouteFilter.ObjectMeta.Namespace, Name: httpRouteFilter.Name}, crHTTPRouteFilter); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get HTTPRouteFilter CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), httpRouteFilter); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create HTTPRouteFilter CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("HTTPRouteFilter CR created: " + httpRouteFilter.Name)
		}
	} else {
		crHTTPRouteFilter.Spec = httpRouteFilter.Spec
		if err := k8sClient.Update(context.Background(), crHTTPRouteFilter); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update HTTPRouteFilter CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("HTTPRouteFilter CR updated: " + httpRouteFilter.Name)
		}
	}
}

// DeployHTTPRouteCR applies the given HttpRoute struct to the Kubernetes cluster.
func DeployHTTPRouteCR(httpRoute *gwapiv1.HTTPRoute, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		httpRoute.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crHTTPRoute := &gwapiv1.HTTPRoute{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: httpRoute.ObjectMeta.Namespace, Name: httpRoute.Name}, crHTTPRoute); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get HTTPRoute CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), httpRoute); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create HTTPRoute CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("HTTPRoute CR created: " + httpRoute.Name)
		}
	} else {
		crHTTPRoute.Spec = httpRoute.Spec
		if err := k8sClient.Update(context.Background(), crHTTPRoute); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update HTTPRoute CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("HTTPRoute CR updated: " + httpRoute.Name)
		}
	}
}

// DeploySecretCR applies the given Secret struct to the Kubernetes cluster.
func DeploySecretCR(secret *corev1.Secret, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		secret.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crSecret := &corev1.Secret{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: secret.ObjectMeta.Namespace, Name: secret.Name}, crSecret); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get Secret CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), secret); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create Secret CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("Secret CR created: " + secret.Name)
		}
	} else {
		crSecret.Data = secret.Data
		if err := k8sClient.Update(context.Background(), crSecret); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update Secret CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("Secret CR updated: " + secret.Name)
		}
	}
}

// DeployBackendCR applies the given Backend struct to the Kubernetes cluster.
func DeployBackendCR(backends *gatewayv1alpha1.Backend, ownerRef *metav1.OwnerReference, k8sClient client.Client) (types.UID, error) {
	if ownerRef != nil {
		backends.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crBackends := &gatewayv1alpha1.Backend{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: backends.ObjectMeta.Namespace, Name: backends.Name}, crBackends); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get Backends CR: " + err.Error())
			return "", err
		}
		if err := k8sClient.Create(context.Background(), backends); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create Backends CR: " + err.Error())
			return "", err
		} else {
			loggers.LoggerK8sClient.Info("Backends CR created: " + backends.Name)
			return backends.ObjectMeta.UID, nil
		}
	} else {
		crBackends.Spec = backends.Spec
		if err := k8sClient.Update(context.Background(), crBackends); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update Backends CR: " + err.Error())
			return "", err
		} else {
			loggers.LoggerK8sClient.Info("Backends CR updated: " + backends.Name)
			return crBackends.ObjectMeta.UID, nil
		}
	}
}

// DeploySecurityPolicyCR applies the given SecurityPolicy struct to the Kubernetes cluster.
func DeploySecurityPolicyCR(securityPolicy *gatewayv1alpha1.SecurityPolicy, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		securityPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crSecurityPolicy := &gatewayv1alpha1.SecurityPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: securityPolicy.ObjectMeta.Namespace, Name: securityPolicy.Name}, crSecurityPolicy); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get SecurityPolicy CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), securityPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create SecurityPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("SecurityPolicy CR created: " + securityPolicy.Name)
		}
	} else {
		crSecurityPolicy.Spec = securityPolicy.Spec
		if err := k8sClient.Update(context.Background(), crSecurityPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update SecurityPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("SecurityPolicy CR updated: " + securityPolicy.Name)
		}
	}
}

// DeployBackendTLSPolicyCR applies the given BackendTLSPolicy struct to the Kubernetes cluster.
func DeployBackendTLSPolicyCR(backendTLSPolicy *gwapiv1a3.BackendTLSPolicy, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		backendTLSPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crBackendTLSPolicy := &gwapiv1a3.BackendTLSPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: backendTLSPolicy.ObjectMeta.Namespace, Name: backendTLSPolicy.Name}, crBackendTLSPolicy); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get BackendTLSPolicy CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), backendTLSPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create BackendTLSPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("BackendTLSPolicy CR created: " + backendTLSPolicy.Name)
		}
	} else {
		crBackendTLSPolicy.Spec = backendTLSPolicy.Spec
		if err := k8sClient.Update(context.Background(), crBackendTLSPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update BackendTLSPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("BackendTLSPolicy CR updated: " + backendTLSPolicy.Name)
		}
	}
}

// DeployRoutePolicyCR applies the given RoutePolicy struct to the Kubernetes cluster.
func DeployRoutePolicyCR(routePolicy *dpv2alpha1.RoutePolicy, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		routePolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crRoutePolicy := &dpv2alpha1.RoutePolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: routePolicy.ObjectMeta.Namespace, Name: routePolicy.Name}, crRoutePolicy); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get RoutePolicy CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), routePolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create RoutePolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("RoutePolicy CR created: " + routePolicy.Name)
		}
	} else {
		crRoutePolicy.Spec = routePolicy.Spec
		if err := k8sClient.Update(context.Background(), crRoutePolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update RoutePolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("RoutePolicy CR updated: " + routePolicy.Name)
		}
	}
}

// DeployEnvoyExtensionPolicyCR applies the given EnvoyExtensionPolicy struct to the Kubernetes cluster.
func DeployEnvoyExtensionPolicyCR(extensionPolicy *gatewayv1alpha1.EnvoyExtensionPolicy, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		extensionPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crExtensionPolicy := &gatewayv1alpha1.EnvoyExtensionPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: extensionPolicy.ObjectMeta.Namespace, Name: extensionPolicy.Name}, crExtensionPolicy); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get EnvoyExtensionPolicy CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), extensionPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create EnvoyExtensionPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("EnvoyExtensionPolicy CR created: " + extensionPolicy.Name)
		}
	} else {
		crExtensionPolicy.Spec = extensionPolicy.Spec
		if err := k8sClient.Update(context.Background(), crExtensionPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update EnvoyExtensionPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("EnvoyExtensionPolicy CR updated: " + extensionPolicy.Name)
		}
	}
}

// DeployBakcendTrafficPolicyCR applies the given BakcendTrafficPolicy struct to the Kubernetes cluster.
func DeployBakcendTrafficPolicyCR(backendTrafficPolicy *gatewayv1alpha1.BackendTrafficPolicy, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		backendTrafficPolicy.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crBackendTrafficPolicy := &gatewayv1alpha1.BackendTrafficPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: backendTrafficPolicy.ObjectMeta.Namespace, Name: backendTrafficPolicy.Name}, crBackendTrafficPolicy); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get BakcendTrafficPolicy CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), backendTrafficPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create BakcendTrafficPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("BakcendTrafficPolicy CR created: " + backendTrafficPolicy.Name)
		}
	} else {
		crBackendTrafficPolicy.Spec = backendTrafficPolicy.Spec
		if err := k8sClient.Update(context.Background(), crBackendTrafficPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update BakcendTrafficPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("BakcendTrafficPolicy CR updated: " + backendTrafficPolicy.Name)
		}
	}
}

// DeployGRPCRouteCR applies the given GRPCRoute struct to the Kubernetes cluster.
func DeployGRPCRouteCR(grpcRoute *gwapiv1a2.GRPCRoute, ownerRef *metav1.OwnerReference, k8sClient client.Client) {
	if ownerRef != nil {
		grpcRoute.ObjectMeta.OwnerReferences = []metav1.OwnerReference{*ownerRef}
	}
	crGRPCRoute := &gwapiv1.GRPCRoute{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: grpcRoute.ObjectMeta.Namespace, Name: grpcRoute.Name}, crGRPCRoute); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get GRPCRoute CR: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), grpcRoute); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create GRPCRoute CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("GRPCRoute CR created: " + grpcRoute.Name)
		}
	} else {
		crGRPCRoute.Spec = grpcRoute.Spec
		if err := k8sClient.Update(context.Background(), crGRPCRoute); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update GRPCRoute CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("GRPCRoute CR updated: " + grpcRoute.Name)
		}
	}
}

// !!! ======== NEW ========

// ====== OLD AI PROVIDER DEPLOY CODE ======

// DeleteAIProviderCR removes the RoutePolicy Custom Resource for the given AIProvider from the Kubernetes cluster based on CR name
func DeleteAIProviderCR(aiProviderName string, k8sClient client.Client) {
	conf, errReadConfig := config.ReadConfigs()
	if errReadConfig != nil {
		loggers.LoggerK8sClient.Errorf("Error reading configurations: %v", errReadConfig)
		return
	}

	crAIProviderRP := &dpv2alpha1.RoutePolicy{}
	// !!! Might have to add some extra logic to get the exact CR
	err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: conf.DataPlane.Namespace, Name: aiProviderName}, crAIProviderRP)
	if err != nil {
		if k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Infof("RoutePolicy CR for the given AI Provider not found: %s", aiProviderName)
		} else {
			loggers.LoggerK8sClient.Error("Unable to get RoutePolicy CR for the given AI Provider: " + err.Error())
		}
		return
	}

	// Proceed to delete the CR if it was successfully retrieved
	err = k8sClient.Delete(context.Background(), crAIProviderRP, &client.DeleteOptions{})
	if err != nil {
		loggers.LoggerK8sClient.Errorf("Unable to delete AI Provider RoutePolicy CR: %v", err)
	} else {
		loggers.LoggerK8sClient.Infof("Deleted AI Provider RoutePolicy CR: %s Successfully", aiProviderName)
	}
}

// ^^^^^^^^^^^^^ OLD AI PROVIDER CODE ^^^^^^^^^^^^^

// ======= OLD RL POLICY CODE =======

// // DeleteAIRatelimitPolicy removes the AIRatelimitPolicy Custom Resource from the Kubernetes cluster based on CR name
// func DeleteAIRatelimitPolicy(airlName string, k8sClient client.Client) {
// 	conf, errReadConfig := config.ReadConfigs()
// 	if errReadConfig != nil {
// 		loggers.LoggerK8sClient.Errorf("Error reading configurations: %v", errReadConfig)
// 		return
// 	}

// 	crAIRatelimitPolicy := &dpv1alpha3.AIRateLimitPolicy{}
// 	err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: conf.DataPlane.Namespace, Name: airlName}, crAIRatelimitPolicy)
// 	if err != nil {
// 		if k8error.IsNotFound(err) {
// 			loggers.LoggerK8sClient.Infof("AIRatelimitPolicy CR not found: %s", airlName)
// 		} else {
// 			loggers.LoggerK8sClient.Error("Unable to get AIRatelimitPolicy CR: " + err.Error())
// 		}
// 		return
// 	}

// 	// Proceed to delete the CR if it was successfully retrieved
// 	err = k8sClient.Delete(context.Background(), crAIRatelimitPolicy, &client.DeleteOptions{})
// 	if err != nil {
// 		loggers.LoggerK8sClient.Errorf("Unable to delete AIRatelimitPolicy CR: %v", err)
// 	} else {
// 		loggers.LoggerK8sClient.Infof("Deleted AIRatelimitPolicy CR: %s Successfully", airlName)
// 	}
// }

// // DeployRateLimitPolicyCR applies the given RateLimitPolicies struct to the Kubernetes cluster.
// func DeployRateLimitPolicyCR(rateLimitPolicies *dpv1alpha1.RateLimitPolicy, k8sClient client.Client) {
// 	crRateLimitPolicies := &dpv1alpha1.RateLimitPolicy{}
// 	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: rateLimitPolicies.ObjectMeta.Namespace, Name: rateLimitPolicies.Name}, crRateLimitPolicies); err != nil {
// 		if !k8error.IsNotFound(err) {
// 			loggers.LoggerK8sClient.Error("Unable to get RateLimitPolicies CR: " + err.Error())
// 		}
// 		if err := k8sClient.Create(context.Background(), rateLimitPolicies); err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to create RateLimitPolicies CR: " + err.Error())
// 		} else {
// 			loggers.LoggerK8sClient.Info("RateLimitPolicies CR created: " + rateLimitPolicies.Name)
// 		}
// 	} else {
// 		crRateLimitPolicies.Spec = rateLimitPolicies.Spec
// 		crRateLimitPolicies.ObjectMeta.Labels = rateLimitPolicies.ObjectMeta.Labels
// 		if err := k8sClient.Update(context.Background(), crRateLimitPolicies); err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to update RateLimitPolicies CR: " + err.Error())
// 		} else {
// 			loggers.LoggerK8sClient.Info("RateLimitPolicies CR updated: " + rateLimitPolicies.Name)
// 		}
// 	}
// }

// UpdateRateLimitPolicyCR applies the updated policy details to all the RateLimitPolicies struct which has the provided label to the Kubernetes cluster.
func UpdateRateLimitPolicyCR(policy eventhubTypes.RateLimitPolicy, k8sClient client.Client) {
	conf, _ := config.ReadConfigs()
	// retrieve all RateLimitPolicies from the Kubernetes cluster with the provided label selector "rateLimitPolicyName"
	rlBackendTrafficPolicyList := &gatewayv1alpha1.BackendTrafficPolicyList{}
	labelMap := map[string]string{"kgw.wso2.com/cpInitiated": "true", "kgw.wso2.com/organization": policy.TenantDomain}
	// Create a list option with the label selector
	listOption := &client.ListOptions{
		Namespace:     conf.DataPlane.Namespace,
		LabelSelector: labels.SelectorFromSet(labelMap),
	}
	err := k8sClient.List(context.Background(), rlBackendTrafficPolicyList, listOption)
	if err != nil {
		loggers.LoggerK8sClient.Errorf("Unable to list BackendTrafficPolicy CRs for Rate Limiting: %v", err)
	}
	if len(rlBackendTrafficPolicyList.Items) == 0 {
		loggers.LoggerK8sClient.Info("No Rate Limit BackendTrafficPolicy CRs found to update")
		return
	}
	loggers.LoggerK8sClient.Debugf("Rate Limit BackendTrafficPolicy CR list retrieved: %+v", rlBackendTrafficPolicyList.Items)
	for _, rlBackendTrafficPolicy := range rlBackendTrafficPolicyList.Items {
		rlBackendTrafficPolicy.Spec.RateLimit.Global.Rules[0].Limit.Requests = uint(policy.DefaultLimit.RequestCount.RequestCount)
		rlBackendTrafficPolicy.Spec.RateLimit.Global.Rules[0].Limit.Unit = gatewayv1alpha1.RateLimitUnit(policy.DefaultLimit.RequestCount.TimeUnit)
		loggers.LoggerK8sClient.Debugf("Rate Limit BackendTrafficPolicy CR updated: %+v", rlBackendTrafficPolicy)
		if err := k8sClient.Update(context.Background(), &rlBackendTrafficPolicy); err != nil {
			loggers.LoggerK8sClient.Errorf("Unable to update Rate Limit BackendTrafficPolicy CR: %+v", err)
		} else {
			loggers.LoggerK8sClient.Infof("Rate Limit BackendTrafficPolicy CR updated: %+v", rlBackendTrafficPolicy.Name)
		}
	}
}

// // DeployAIRateLimitPolicyCR applies the given AIRateLimitPolicies struct to the Kubernetes cluster.
// func DeployAIRateLimitPolicyCR(aiRateLimitPolicies *dpv1alpha3.AIRateLimitPolicy, k8sClient client.Client) {
// 	crAIRateLimitPolicies := &dpv1alpha3.AIRateLimitPolicy{}
// 	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: aiRateLimitPolicies.ObjectMeta.Namespace, Name: aiRateLimitPolicies.Name}, crAIRateLimitPolicies); err != nil {
// 		if !k8error.IsNotFound(err) {
// 			loggers.LoggerK8sClient.Error("Unable to get RateLimitPolicies CR: " + err.Error())
// 		}
// 		if err := k8sClient.Create(context.Background(), aiRateLimitPolicies); err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to create RateLimitPolicies CR: " + err.Error())
// 		} else {
// 			loggers.LoggerK8sClient.Info("RateLimitPolicies CR created: " + aiRateLimitPolicies.Name)
// 		}
// 	} else {
// 		crAIRateLimitPolicies.Spec = aiRateLimitPolicies.Spec
// 		crAIRateLimitPolicies.ObjectMeta.Labels = aiRateLimitPolicies.ObjectMeta.Labels
// 		if err := k8sClient.Update(context.Background(), crAIRateLimitPolicies); err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to update RateLimitPolicies CR: " + err.Error())
// 		} else {
// 			loggers.LoggerK8sClient.Info("RateLimitPolicies CR updated: " + aiRateLimitPolicies.Name)
// 		}
// 	}
// }

// DeploySubscriptionRateLimitPolicyCR applies the given RateLimitPolicies struct to the Kubernetes cluster.
func DeploySubscriptionRateLimitPolicyCR(policy eventhubTypes.SubscriptionPolicy, k8sClient client.Client) {
	conf, _ := config.ReadConfigs()
	crRLBackendTrafficPolicy := gatewayv1alpha1.BackendTrafficPolicy{}
	crName := utils.CreateSubscriptionPolicyName(policy.Name, policy.TenantDomain)

	unit, requestsPerUnit := getRateLimitPolicyContents(policy)
	loggers.LoggerK8sClient.Infof("Requests Per Unit after parsing: %d | Unit: %s", requestsPerUnit, unit)

	gatewayName, _ := getGatewayNameFromK8s(k8sClient)
	loggers.LoggerK8sClient.Infof("Gateway Name fetched from the k8s cluster: %s", gatewayName)
	if gatewayName == "" {
		gatewayName = "wso2-kgw-default"
	}
	labelMap := map[string]string{
		"kgw.wso2.com/cpInitiated": "true",
		"kgw.wso2.com/cpName":      policy.Name,
	}

	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: conf.DataPlane.Namespace, Name: crName}, &crRLBackendTrafficPolicy); err != nil {
		crRLBackendTrafficPolicy = gatewayv1alpha1.BackendTrafficPolicy{
			ObjectMeta: metav1.ObjectMeta{
				Name:      crName,
				Namespace: conf.DataPlane.Namespace,
				Labels:    labelMap,
			},
			Spec: gatewayv1alpha1.BackendTrafficPolicySpec{
				MergeType: ptr.To(gatewayv1alpha1.MergeType("StrategicMerge")),
				RateLimit: &gatewayv1alpha1.RateLimitSpec{
					Type: gatewayv1alpha1.RateLimitType("Global"),
					Global: &gatewayv1alpha1.GlobalRateLimit{
						Rules: []gatewayv1alpha1.RateLimitRule{
							{
								ClientSelectors: []gatewayv1alpha1.RateLimitSelectCondition{
									{
										Headers: []gatewayv1alpha1.HeaderMatch{
											{
												Name:   "x-wso2-api-id",
												Value:  ptr.To(""),
												Invert: ptr.To(false),
											},
											{
												Name:   "x-wso2-organization",
												Value:  ptr.To(""),
												Invert: ptr.To(false),
											},
											{
												Name:   "x-wso2-subscription-id",
												Value:  ptr.To(""),
												Invert: ptr.To(false),
											},
											{
												Name:   "policy-id",
												Value:  &policy.Name,
												Invert: ptr.To(false),
											},
										},
									},
								},
								Limit: gatewayv1alpha1.RateLimitValue{
									Requests: requestsPerUnit,
									Unit:     unit,
								},
								Shared: ptr.To(false),
							},
						},
					},
				},
				PolicyTargetReferences: gatewayv1alpha1.PolicyTargetReferences{
					TargetRefs: []gwapiv1a2.LocalPolicyTargetReferenceWithSectionName{
						{
							LocalPolicyTargetReference: gwapiv1a2.LocalPolicyTargetReference{
								Group: gwapiv1a2.Group(constants.GatewayGroup),
								Kind:  gwapiv1a2.Kind("Gateway"),
								Name:  gwapiv1a2.ObjectName(gatewayName),
							},
						},
					},
				},
			},
		}
		if err := k8sClient.Create(context.Background(), &crRLBackendTrafficPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create RateLimit BackendTrafficPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("RateLimit BackendTrafficPolicy CR created: " + crRLBackendTrafficPolicy.Name)
		}
	} else {
		crRLBackendTrafficPolicy.Spec.RateLimit.Global.Rules[0].Limit.Requests = requestsPerUnit
		crRLBackendTrafficPolicy.Spec.RateLimit.Global.Rules[0].Limit.Unit = unit
		if err := k8sClient.Update(context.Background(), &crRLBackendTrafficPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update RateLimit BackendTrafficPolicy CR: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("RateLimit BackendTrafficPolicy CR updated: " + crRLBackendTrafficPolicy.Name)
		}
	}

}

// DeployAIRateLimitPolicyFromCPPolicy applies the given AIRateLimitPolicies struct to the Kubernetes cluster.
func DeployAIRateLimitPolicyFromCPPolicy(policy eventhubTypes.SubscriptionPolicy, k8sClient client.Client) {
	conf, _ := config.ReadConfigs()
	unit, requestsPerUnit := getRateLimitPolicyContents(policy)
	loggers.LoggerK8sClient.Infof("Requests Per Unit after parsing: %d | Unit: %s", requestsPerUnit, unit)
	gatewayName, _ := getGatewayNameFromK8s(k8sClient)
	loggers.LoggerK8sClient.Infof("Gateway Name fetched from the k8s cluster: %s", gatewayName)
	if gatewayName == "" {
		gatewayName = "wso2-kgw-default"
	}

	labelMap := map[string]string{
		"kgw.wso2.com/cpInitiated": "true",
		"kgw.wso2.com/cpName":      policy.Name,
	}

	crRLBackendTrafficPolicy := gatewayv1alpha1.BackendTrafficPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      PrepareSubscritionPolicyCRName(policy.Name, policy.TenantDomain),
			Namespace: conf.DataPlane.Namespace,
			Labels:    labelMap,
		},

		Spec: gatewayv1alpha1.BackendTrafficPolicySpec{
			MergeType: ptr.To(gatewayv1alpha1.MergeType("StrategicMerge")),
			RateLimit: &gatewayv1alpha1.RateLimitSpec{
				Type: gatewayv1alpha1.RateLimitType("Global"),
				Global: &gatewayv1alpha1.GlobalRateLimit{
					Rules: []gatewayv1alpha1.RateLimitRule{
						{
							ClientSelectors: []gatewayv1alpha1.RateLimitSelectCondition{
								{
									Headers: []gatewayv1alpha1.HeaderMatch{
										{
											Name:   "x-wso2-api-id",
											Value:  ptr.To(""),
											Invert: ptr.To(false),
										},
										{
											Name:   "x-wso2-organization",
											Value:  ptr.To(""),
											Invert: ptr.To(false),
										},
										{
											Name:   "x-wso2-subscription-id",
											Value:  ptr.To(""),
											Invert: ptr.To(false),
										},
										{
											Name:   "cost",
											Value:  ptr.To(""),
											Invert: ptr.To(false),
										},
										{
											Name:   "policy-id",
											Value:  &policy.Name,
											Invert: ptr.To(false),
										},
									},
								},
							},
							Limit: gatewayv1alpha1.RateLimitValue{
								Requests: requestsPerUnit,
								Unit:     unit,
							},
							Shared: ptr.To(false),
						},
					},
				},
			},
			PolicyTargetReferences: gatewayv1alpha1.PolicyTargetReferences{
				TargetRefs: []gwapiv1a2.LocalPolicyTargetReferenceWithSectionName{
					{
						LocalPolicyTargetReference: gwapiv1a2.LocalPolicyTargetReference{
							Group: gwapiv1a2.Group(constants.GatewayGroup),
							Kind:  gwapiv1a2.Kind("Gateway"),
							Name:  gwapiv1a2.ObjectName(gatewayName),
						},
					},
				},
			},
		},
	}
	crRLBackendTrafficPolicyFetched := &gatewayv1alpha1.BackendTrafficPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: crRLBackendTrafficPolicy.ObjectMeta.Namespace, Name: crRLBackendTrafficPolicy.Name}, crRLBackendTrafficPolicyFetched); err != nil {
		if !k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Error("Unable to get BackendTrafficPolicy CR for AI RateLimit: " + err.Error())
		}
		if err := k8sClient.Create(context.Background(), &crRLBackendTrafficPolicy); err != nil {
			loggers.LoggerK8sClient.Error("Unable to create BackendTrafficPolicy CR for AI RateLimit: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("BackendTrafficPolicy CR created for AI Ratelimit: " + crRLBackendTrafficPolicy.Name)
		}
	} else {
		crRLBackendTrafficPolicyFetched.Spec = crRLBackendTrafficPolicy.Spec
		crRLBackendTrafficPolicyFetched.ObjectMeta.Labels = crRLBackendTrafficPolicy.ObjectMeta.Labels
		if err := k8sClient.Update(context.Background(), crRLBackendTrafficPolicyFetched); err != nil {
			loggers.LoggerK8sClient.Error("Unable to update BackendTrafficPolicy CR for AI RateLimit: " + err.Error())
		} else {
			loggers.LoggerK8sClient.Info("BackendTrafficPolicy CR updated for AI Ratelimit: " + crRLBackendTrafficPolicyFetched.Name)
		}
	}
}

// !!!TODO: Might be possible to use single method for both SubscriptionRL and SubscriptionAIRL(because both use BackendTrafficPolicy CR)
// UnDeploySubscriptionRateLimitPolicyCR deletes the given RateLimit BackendTrafficPolicy struct from the Kubernetes cluster.
func UnDeploySubscriptionRateLimitPolicyCR(crName string, k8sClient client.Client) {
	conf, _ := config.ReadConfigs()
	crRLBackendTrafficPPolicies := &gatewayv1alpha1.BackendTrafficPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: conf.DataPlane.Namespace, Name: crName}, crRLBackendTrafficPPolicies); err != nil {
		loggers.LoggerK8sClient.Error("Unable to get RateLimit BackendTrafficPolicies CR: " + err.Error())
	}
	err := k8sClient.Delete(context.Background(), crRLBackendTrafficPPolicies, &client.DeleteOptions{})
	if err != nil {
		loggers.LoggerK8sClient.Error("Unable to delete RateLimit BackendTrafficPolicy CR: " + err.Error())
	}
	loggers.LoggerK8sClient.Debug("RateLimit BackendTrafficPolicy CR deleted: " + crRLBackendTrafficPPolicies.Name)
}

// UndeploySubscriptionAIRateLimitPolicyCR deletes the given AIRateLimit BackendTrafficPolicy struct from the Kubernetes cluster.
func UndeploySubscriptionAIRateLimitPolicyCR(crName string, k8sClient client.Client) {
	conf, _ := config.ReadConfigs()
	crAIRLBackendTrafficPolicies := &gatewayv1alpha1.BackendTrafficPolicy{}
	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: conf.DataPlane.Namespace, Name: crName}, crAIRLBackendTrafficPolicies); err != nil {
		loggers.LoggerK8sClient.Error("Unable to get Subscription AI RateLimit BackendTrafficPolicy CR: " + err.Error())
	}
	err := k8sClient.Delete(context.Background(), crAIRLBackendTrafficPolicies, &client.DeleteOptions{})
	if err != nil {
		loggers.LoggerK8sClient.Error("Unable to delete Subscription AI RateLimit BackendTrafficPolicy CR: " + err.Error())
	}
	loggers.LoggerK8sClient.Debug("Subscription AI RateLimit BackendTrafficPolicy CR deleted: " + crAIRLBackendTrafficPolicies.Name)
}

// ^^^^^^^^^^^^^ OLD RL POLICY CODE ^^^^^^^^^^^^^

// ====== OLD TOKEN ISSUER DEPLOY CODE =====

// // !!!TODO: Have to change this becuase now we use SecurityPolicy for this
// // CreateAndUpdateTokenIssuersCR applies the given TokenIssuers struct to the Kubernetes cluster.
// func CreateAndUpdateTokenIssuersCR(keyManager eventhubTypes.ResolvedKeyManager, k8sClient client.Client) error {
// 	conf, _ := config.ReadConfigs()
// 	sha1ValueofKmName := GetSha1Value(keyManager.Name)
// 	sha1ValueOfOrganization := GetSha1Value(keyManager.Organization)
// 	labelMap := map[string]string{"name": sha1ValueofKmName,
// 		"organization": sha1ValueOfOrganization,
// 		"InitiateFrom": "CP",
// 	}
// 	tokenIssuer := dpv1alpha2.TokenIssuer{
// 		ObjectMeta: metav1.ObjectMeta{Name: keyManager.UUID,
// 			Namespace: conf.DataPlane.Namespace,
// 			Labels:    labelMap,
// 		},
// 		Spec: dpv1alpha2.TokenIssuerSpec{
// 			Name:                keyManager.Name,
// 			Organization:        keyManager.Organization,
// 			Issuer:              keyManager.KeyManagerConfig.Issuer,
// 			ClaimMappings:       marshalClaimMappings(keyManager.KeyManagerConfig.ClaimMappings),
// 			SignatureValidation: marshalSignatureValidation(keyManager.KeyManagerConfig),
// 			TargetRef:           &v1alpha2.NamespacedPolicyTargetReference{Group: constants.GatewayGroup, Kind: constants.GatewayKind, Name: constants.GatewayName},
// 		},
// 	}
// 	tokenIssuer.Spec.ConsumerKeyClaim = constants.ConsumerKeyClaim
// 	if keyManager.KeyManagerConfig.ConsumerKeyClaim != "" {
// 		tokenIssuer.Spec.ConsumerKeyClaim = keyManager.KeyManagerConfig.ConsumerKeyClaim
// 	}
// 	keyManager.KeyManagerConfig.ScopesClaim = constants.ScopesClaim
// 	if keyManager.KeyManagerConfig.ScopesClaim != "" {
// 		tokenIssuer.Spec.ScopesClaim = keyManager.KeyManagerConfig.ScopesClaim
// 	}
// 	crTokenIssuer := &dpv1alpha2.TokenIssuer{}
// 	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: tokenIssuer.ObjectMeta.Namespace, Name: tokenIssuer.Name}, crTokenIssuer); err != nil {
// 		if !k8error.IsNotFound(err) {
// 			loggers.LoggerK8sClient.Error("Unable to get TokenIssuer CR: " + err.Error())
// 		}
// 		err := k8sClient.Create(context.Background(), &tokenIssuer)
// 		if err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to create TokenIssuer CR: " + err.Error())
// 			return err
// 		}
// 		loggers.LoggerK8sClient.Infof("TokenIssuer CR created: " + tokenIssuer.Name)
// 	} else {
// 		crTokenIssuer.Spec = tokenIssuer.Spec
// 		if err := k8sClient.Update(context.Background(), crTokenIssuer); err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to update TokenIssuer CR: " + err.Error())
// 		} else {
// 			loggers.LoggerK8sClient.Info("TokenIssuer CR updated: " + tokenIssuer.Name)
// 		}
// 	}
// 	internalKeyTokenIssuer := dpv1alpha2.TokenIssuer{
// 		ObjectMeta: metav1.ObjectMeta{Name: keyManager.Organization + constants.InternalKeySuffix,
// 			Namespace: conf.DataPlane.Namespace,
// 			Labels:    labelMap,
// 		},
// 		Spec: dpv1alpha2.TokenIssuerSpec{
// 			Name:          constants.InternalKeyTokenIssuerName,
// 			Organization:  keyManager.Organization,
// 			Issuer:        conf.ControlPlane.InternalKeyIssuer,
// 			ClaimMappings: marshalClaimMappings(keyManager.KeyManagerConfig.ClaimMappings),
// 			SignatureValidation: &dpv1alpha2.SignatureValidation{
// 				Certificate: &dpv1alpha2.CERTConfig{
// 					SecretRef: &dpv1alpha2.RefConfig{
// 						Name: constants.InternalKeySecretName,
// 						Key:  constants.InternalKeySecretKey,
// 					},
// 				},
// 			},
// 			TargetRef: &v1alpha2.NamespacedPolicyTargetReference{Group: constants.GatewayGroup, Kind: constants.GatewayKind, Name: constants.GatewayName},
// 		},
// 	}
// 	internalKeyTokenIssuer.Spec.ConsumerKeyClaim = constants.ConsumerKeyClaim
// 	internalKeyTokenIssuer.Spec.ScopesClaim = constants.ScopesClaim
// 	crInternalTokenIssuer := &dpv1alpha2.TokenIssuer{}
// 	if err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: internalKeyTokenIssuer.ObjectMeta.Namespace, Name: internalKeyTokenIssuer.Name}, crInternalTokenIssuer); err != nil {
// 		if !k8error.IsNotFound(err) {
// 			loggers.LoggerK8sClient.Error("Unable to get Internal TokenIssuer CR: " + err.Error())
// 		}
// 		err = k8sClient.Create(context.Background(), &internalKeyTokenIssuer)
// 		if err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to create Internal TokenIssuer CR: " + err.Error())
// 			return err
// 		}
// 		loggers.LoggerK8sClient.Infof("Internal TokenIssuer CR created: " + internalKeyTokenIssuer.Name)
// 	} else {
// 		crInternalTokenIssuer.Spec = internalKeyTokenIssuer.Spec
// 		if err := k8sClient.Update(context.Background(), crInternalTokenIssuer); err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to update Internal TokenIssuer CR: " + err.Error())
// 		} else {
// 			loggers.LoggerK8sClient.Info("TokenIssuer CR updated: " + internalKeyTokenIssuer.Name)
// 		}
// 	}
// 	return nil
// }

// // DeleteTokenIssuerCR deletes the TokenIssuer struct from the Kubernetes cluster.
// func DeleteTokenIssuerCR(k8sClient client.Client, tokenIssuer dpv1alpha2.TokenIssuer) error {
// 	// Skip the deletion if the token issuer is for internal keys
// 	if !strings.Contains(tokenIssuer.Name, constants.InternalKeySuffix) {
// 		err := k8sClient.Delete(context.Background(), &tokenIssuer, &client.DeleteOptions{})
// 		if err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to delete TokenIssuer CR: " + err.Error())
// 			return err
// 		}
// 		loggers.LoggerK8sClient.Debug("TokenIssuer CR deleted: " + tokenIssuer.Name)
// 	}
// 	return nil
// }

// // DeleteTokenIssuersCR deletes the TokenIssuers struct from the Kubernetes cluster.
// func DeleteTokenIssuersCR(k8sClient client.Client, keymanagerName string, tenantDomain string) error {
// 	conf, _ := config.ReadConfigs()
// 	sha1ValueofKmName := GetSha1Value(keymanagerName)
// 	sha1ValueOfOrganization := GetSha1Value(tenantDomain)
// 	labelMap := map[string]string{"name": sha1ValueofKmName, "organization": sha1ValueOfOrganization}
// 	// Create a list option with the label selector
// 	listOption := &client.ListOptions{
// 		Namespace:     conf.DataPlane.Namespace,
// 		LabelSelector: labels.SelectorFromSet(labelMap),
// 	}
// 	tokenIssuerList := &dpv1alpha2.TokenIssuerList{}
// 	err := k8sClient.List(context.Background(), tokenIssuerList, listOption)
// 	if err != nil {
// 		loggers.LoggerK8sClient.Error("Unable to list TokenIssuer CR: " + err.Error())
// 	}
// 	if len(tokenIssuerList.Items) == 0 {
// 		loggers.LoggerK8sClient.Debug("No TokenIssuer CR found for deletion")
// 	}
// 	for _, tokenIssuer := range tokenIssuerList.Items {
// 		err := DeleteTokenIssuerCR(k8sClient, tokenIssuer)
// 		if err != nil {
// 			loggers.LoggerK8sClient.Error("Unable to delete TokenIssuer CR: " + err.Error())
// 			return err
// 		}
// 		loggers.LoggerK8sClient.Debug("TokenIssuer CR deleted: " + tokenIssuer.Name)
// 	}
// 	return nil
// }

// DeleteSecurityPolicyCRs deletes the SecurityPolicy CRs for the given key manager.
// !!!TODO: Change the logic to remove the provider details from all the SPs if provider name is equal to the key manager name
func UpdateSecurityPolicyCRs(keymanagerName string, tenantDomain string, k8sClient client.Client, removeProvider bool) error {
	conf, _ := config.ReadConfigs()
	kmCache := cache.GetKeyManagerCacheInstance()
	kmNameWithOrg := tenantDomain + "-" + keymanagerName
	loggers.LoggerK8sClient.Debugf("Checking for SecurityPolicy CRs for the org: %s", tenantDomain)
	// SecurityPolicy CRs are filtered by organization
	labelMap := map[string]string{"kgw.wso2.com/organization": tenantDomain}
	listOption := &client.ListOptions{
		Namespace:     conf.DataPlane.Namespace,
		LabelSelector: labels.SelectorFromSet(labelMap),
	}

	securityPolicyList := &gatewayv1alpha1.SecurityPolicyList{}
	err := k8sClient.List(context.Background(), securityPolicyList, listOption)
	if err != nil {
		loggers.LoggerK8sClient.Error("Unable to list SecurityPolicy CR: " + err.Error())
	}
	if len(securityPolicyList.Items) == 0 {
		loggers.LoggerK8sClient.Infof("No SecurityPolicy CR found for update")
	}
	for _, securitypolicy := range securityPolicyList.Items {
		providers := securitypolicy.Spec.JWT.Providers
		loggers.LoggerK8sClient.Debugf("Providers: %+v", providers)
		updated := false    // Track if any changes were made
		if removeProvider { // Remove the provider details from JWT segment in each CR
			loggers.LoggerK8sClient.Infof("Removing the provider from the SecurityPolicy")
			updatedProviders := []gatewayv1alpha1.JWTProvider{}
			for _, provider := range providers {
				if provider.Name != kmNameWithOrg {
					updatedProviders = append(updatedProviders, provider)
				} else {
					updated = true // Mark that we removed a provider
				}
			}
			securitypolicy.Spec.JWT.Providers = updatedProviders
			loggers.LoggerK8sClient.Infof("Removed the %s provider details from JWT segment in SecurityPolicy CR: %s", keymanagerName, securitypolicy.Name)
		} else { // Update the provider details in the CRs
			for i, provider := range providers {
				if provider.Name == kmNameWithOrg {
					updatedKMFromAPIM, exists := kmCache.GetKeyManager(keymanagerName, tenantDomain)

					if !exists {
						loggers.LoggerK8sClient.Errorf("KeyManager '%s' not found in cache", keymanagerName)
						continue
					}
					// Update provider details
					securitypolicy.Spec.JWT.Providers[i].Issuer = updatedKMFromAPIM.ResolvedKM.KeyManagerConfig.Issuer

					if updatedKMFromAPIM.ResolvedKM.KeyManagerConfig.CertificateType == "JWKS" {
						securitypolicy.Spec.JWT.Providers[i].RemoteJWKS.URI = updatedKMFromAPIM.ResolvedKM.KeyManagerConfig.CertificateValue
					} else if updatedKMFromAPIM.ResolvedKM.KeyManagerConfig.CertificateType == "PEM" {
						//!!! TODO: Currently the Envoy SecurityPolicy doesn't support mounting certificates.
						// Hence the implementation is omitted for now.
						loggers.LoggerK8sClient.Warnf("PEM certificates are not supported in the Envoy SecurityPolicy CRs. Hence skipping the new configuration.")
					}
					mappedClaims := []gatewayv1alpha1.ClaimToHeader{}
					for _, claim := range updatedKMFromAPIM.ResolvedKM.KeyManagerConfig.ClaimMappings {
						mappedClaims = append(mappedClaims, gatewayv1alpha1.ClaimToHeader{
							Header: claim.LocalClaim,
							Claim:  claim.RemoteClaim,
						})
					}
					securitypolicy.Spec.JWT.Providers[i].ClaimToHeaders = mappedClaims
					updated = true
				}
			}
			loggers.LoggerK8sClient.Infof("Updated the provider details in the security policy CR: %s", securitypolicy.Name)
		}
		// Update the SecurityPolicy CR in the cluster if any changes were made
		if updated {
			if err := k8sClient.Update(context.Background(), &securitypolicy); err != nil {
				loggers.LoggerK8sClient.Errorf("Unable to update SecurityPolicy CR '%s': %v", securitypolicy.Name, err)
				continue // Continue with other policies even if one fails
			}
			loggers.LoggerK8sClient.Infof("Successfully updated SecurityPolicy CR: %s", securitypolicy.Name)
		} else {
			loggers.LoggerK8sClient.Infof("No changes needed for SecurityPolicy CR: %s", securitypolicy.Name)
		}
	}
	return nil
}

func DeleteSecurityPolicyCR(k8sClient client.Client, securityPolicy gatewayv1alpha1.SecurityPolicy) error {
	// !!!NOTE: Previously we had a check to stop deleting the token issuer if it is for internal keys
	// Are we going to do the same for this as well?
	if !strings.Contains(securityPolicy.Name, constants.InternalKeySuffix) {
		err := k8sClient.Delete(context.Background(), &securityPolicy, &client.DeleteOptions{})
		if err != nil {
			loggers.LoggerK8sClient.Error("Unable to delete SecurityPolicy CR: " + err.Error())
			return err
		}
		loggers.LoggerK8sClient.Debug("SecurityPolicy CR deleted: " + securityPolicy.Name)
	}
	return nil
}

// // UpdateTokenIssuersCR applies the given TokenIssuers struct to the Kubernetes cluster.
// func UpdateTokenIssuersCR(keyManager eventhubTypes.ResolvedKeyManager, k8sClient client.Client) error {
// 	conf, _ := config.ReadConfigs()
// 	sha1ValueofKmName := GetSha1Value(keyManager.Name)
// 	sha1ValueOfOrganization := GetSha1Value(keyManager.Organization)
// labelMap := map[string]string{"name": sha1ValueofKmName, "organization": sha1ValueOfOrganization}
// tokenIssuer := &dpv1alpha2.TokenIssuer{}
// err := k8sClient.Get(context.Background(), client.ObjectKey{Name: keyManager.UUID, Namespace: conf.DataPlane.Namespace}, tokenIssuer)
// if err != nil {
// 	loggers.LoggerK8sClient.Error("Unable to get TokenIssuer CR: " + err.Error())
// 	return err
// }
// 	tokenIssuer.ObjectMeta.Labels = labelMap
// 	tokenIssuer.Spec.Name = keyManager.Name
// 	tokenIssuer.Spec.Organization = keyManager.Organization
// 	tokenIssuer.Spec.Issuer = keyManager.KeyManagerConfig.Issuer
// 	tokenIssuer.Spec.ClaimMappings = marshalClaimMappings(keyManager.KeyManagerConfig.ClaimMappings)
// 	tokenIssuer.Spec.SignatureValidation = marshalSignatureValidation(keyManager.KeyManagerConfig)
// 	tokenIssuer.Spec.TargetRef = &v1alpha2.NamespacedPolicyTargetReference{Group: constants.GatewayGroup, Kind: constants.GatewayKind, Name: constants.GatewayName}
// 	if keyManager.KeyManagerConfig.ConsumerKeyClaim != "" {
// 		tokenIssuer.Spec.ConsumerKeyClaim = keyManager.KeyManagerConfig.ConsumerKeyClaim
// 	}
// 	if keyManager.KeyManagerConfig.ScopesClaim != "" {
// 		tokenIssuer.Spec.ScopesClaim = keyManager.KeyManagerConfig.ScopesClaim
// 	}
// 	err = k8sClient.Update(context.Background(), tokenIssuer)
// 	if err != nil {
// 		loggers.LoggerK8sClient.Error("Unable to update TokenIssuer CR: " + err.Error())
// 		return err
// 	}
// 	loggers.LoggerK8sClient.Debug("TokenIssuer CR updated: " + tokenIssuer.Name)
// 	return nil
// }

// UpdateSecurityPolicyCR updates the SecurityPolicy CR for the given key manager.
func UpdateSecurityPolicyCR(keyManager eventhubTypes.ResolvedKeyManager, k8sClient client.Client) error {
	conf, _ := config.ReadConfigs()
	sha1ValueofKmName := utils.GetSha1Value(keyManager.Name)
	sha1ValueOfOrganization := utils.GetSha1Value(keyManager.Organization)

	// Filter SecurityPolicies by Key Manager labels
	labelMap := map[string]string{
		"name":         sha1ValueofKmName,
		"organization": sha1ValueOfOrganization,
		"managed-by":   "kgw",         // Additional filter for Key Manager managed policies
		"policy-type":  "key-manager", // Distinguish from other SecurityPolicy types
	}

	// Create a list option with the label selector
	listOption := &client.ListOptions{
		Namespace:     conf.DataPlane.Namespace,
		LabelSelector: labels.SelectorFromSet(labelMap),
	}

	securityPolicyList := &gatewayv1alpha1.SecurityPolicyList{}
	err := k8sClient.List(context.Background(), securityPolicyList, listOption)
	if err != nil {
		loggers.LoggerK8sClient.Errorf("Unable to list SecurityPolicy CRs for Key Manager '%s': %v", keyManager.Name, err)
		return err
	}
	if len(securityPolicyList.Items) == 0 {
		loggers.LoggerK8sClient.Infof("No SecurityPolicy CRs found for Key Manager '%s' to update", keyManager.Name)
		return nil
	}

	// Update each SecurityPolicy CR with new JWT provider configuration
	for _, securityPolicy := range securityPolicyList.Items {
		if err := updateSecurityPolicyJWTConfig(&securityPolicy, keyManager, k8sClient); err != nil {
			loggers.LoggerK8sClient.Errorf("Failed to update SecurityPolicy CR '%s' for Key Manager '%s': %v",
				securityPolicy.Name, keyManager.Name, err)
			continue
		}
		loggers.LoggerK8sClient.Infof("SecurityPolicy CR '%s' updated for Key Manager '%s'",
			securityPolicy.Name, keyManager.Name)
	}
	loggers.LoggerK8sClient.Debug("SecurityPolicy CR updated")
	return nil
}

// updateSecurityPolicyJWTConfig is a helper function that updates the JWT configuration for the given key manager.
func updateSecurityPolicyJWTConfig(securityPolicy *gatewayv1alpha1.SecurityPolicy, keyManager eventhubTypes.ResolvedKeyManager, k8sClient client.Client) error {
	securitypolicy := &gatewayv1alpha1.SecurityPolicy{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: securityPolicy.Namespace, Name: securityPolicy.Name}, securitypolicy)
	if err != nil {
		return fmt.Errorf("failed to get current SecurityPolicy: %v", err)
	}

	if securitypolicy.Spec.JWT == nil {
		loggers.LoggerK8sClient.Infof("No JWT configuration found for SecurityPolicy '%s', creating new one", securityPolicy.Name)
	} else {
		// Since there can be multiple providers, we need to update the one that belongs to the key manager
		for _, provider := range securitypolicy.Spec.JWT.Providers {
			if provider.Name == keyManager.Name {
				provider.Issuer = keyManager.KeyManagerConfig.Issuer
				provider.RemoteJWKS.URI = keyManager.KeyManagerConfig.CertificateValue
			}
		}
	}
	err = k8sClient.Update(context.Background(), securitypolicy)
	if err != nil {
		loggers.LoggerK8sClient.Error("Unable to update SecurityPolicy CR: " + err.Error())
		return err
	}

	return nil
}

// New UpdateSecurityPolicyCR function which uses .Get()
// func UpdateSecurityPolicyCR(keyManager eventhubTypes.ResolvedKeyManager, k8sClient client.Client) error {
//     conf, _ := config.ReadConfigs()
//     sha1ValueofKmName := GetSha1Value(keyManager.Name)
//     sha1ValueOfOrganization := GetSha1Value(keyManager.Organization)
//     labelMap := map[string]string{"name": sha1ValueofKmName, "organization": sha1ValueOfOrganization}
//     securityPolicy := &gatewayv1alpha1.SecurityPolicy{}
//     err := k8sClient.Get(context.Background(), client.ObjectKey{Name: keyManager.UUID, Namespace: conf.DataPlane.Namespace}, securityPolicy)
//     if err != nil {
//         loggers.LoggerK8sClient.Error("Unable to get SecurityPolicy CR for Given KM: " + err.Error())
//         return err
//     }
//     securityPolicy.ObjectMeta.Labels = labelMap
//     if securityPolicy.Spec.JWT == nil {
//         loggers.LoggerK8sClient.Infof("No JWT configuration found for SecurityPolicy '%s', creating new one", securityPolicy.Name)
//     } else {
//         // Since there can be multiple providers, we need to update the one that belongs to the key manager
//         for _, provider := range securityPolicy.Spec.JWT.Providers {
//             if provider.Name == keyManager.Name {
//                 provider.Issuer = keyManager.KeyManagerConfig.Issuer
//                 provider.RemoteJWKS.URI = keyManager.KeyManagerConfig.CertificateValue
//             }
//         }
//     }
//     err = k8sClient.Update(context.Background(), securityPolicy)
//     if err != nil {
//         loggers.LoggerK8sClient.Error("Unable to update SecurityPolicy CR with given KM Details: " + err.Error())
//         return err
//     }
//     loggers.LoggerK8sClient.Debug("SecurityPolicy CR updated")
//     return nil
// }

//^^^^^^^^^^^^^^^^^^OLD TOKEN ISSUER CODE^^^^^^^^^^^^^^^^^^^

// func marshalSignatureValidation(keyManagerConfig eventhubTypes.KeyManagerConfig) *dpv1alpha2.SignatureValidation {
// 	if keyManagerConfig.CertificateType != "" && keyManagerConfig.CertificateValue != "" {
// 		if keyManagerConfig.CertificateType == "JWKS" {
// 			return &dpv1alpha2.SignatureValidation{JWKS: &dpv1alpha2.JWKS{URL: keyManagerConfig.CertificateValue}}
// 		}
// 		return &dpv1alpha2.SignatureValidation{Certificate: &dpv1alpha2.CERTConfig{CertificateInline: &keyManagerConfig.CertificateValue}}
// 	}
// 	return nil
// }

//	func marshalClaimMappings(claimMappings []eventhubTypes.Claim) *[]dpv1alpha2.ClaimMapping {
//		resolvedClaimMappings := make([]dpv1alpha2.ClaimMapping, 0)
//		for _, claim := range claimMappings {
//			resolvedClaimMappings = append(resolvedClaimMappings, dpv1alpha2.ClaimMapping{RemoteClaim: claim.RemoteClaim, LocalClaim: claim.LocalClaim})
//		}
//		return &resolvedClaimMappings
//	}

// // !!!TODO: Change this because now there's now API CRs. We use RouteMetadata CRs instead.
// // RetrieveAllAPISFromK8s retrieves all the API CRs from the Kubernetes cluster
// func RetrieveAllAPISFromK8s(k8sClient client.Client, nextToken string) ([]dpv1alpha3.API, string, error) {
// 	conf, _ := config.ReadConfigs()
// 	apiList := dpv1alpha3.APIList{}
// 	resolvedAPIList := make([]dpv1alpha3.API, 0)
// 	var err error
// 	if nextToken == "" {
// 		err = k8sClient.List(context.Background(), &apiList, &client.ListOptions{Namespace: conf.DataPlane.Namespace})
// 	} else {
// 		err = k8sClient.List(context.Background(), &apiList, &client.ListOptions{Namespace: conf.DataPlane.Namespace, Continue: nextToken})
// 	}
// 	if err != nil {
// 		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "Failed to get application from k8s %v", err.Error()))
// 		return nil, "", err
// 	}
// 	resolvedAPIList = append(resolvedAPIList, apiList.Items...)
// 	if apiList.Continue != "" {
// 		tempAPIList, _, err := RetrieveAllAPISFromK8s(k8sClient, apiList.Continue)
// 		if err != nil {
// 			return nil, "", err
// 		}
// 		resolvedAPIList = append(resolvedAPIList, tempAPIList...)
// 	}
// 	return resolvedAPIList, apiList.Continue, nil
// }

// RetrieveAllRouteMetasFromK8s retrieves all the RouteMetadata CRs(APIs) from the Kubernetes cluster
func RetrieveAllRouteMetasFromK8s(k8sClient client.Client, nextToken string) ([]dpv2alpha1.RouteMetadata, string, error) {
	conf, _ := config.ReadConfigs()
	routeMetaList := dpv2alpha1.RouteMetadataList{}
	err := k8sClient.List(context.Background(), &routeMetaList, &client.ListOptions{
		Namespace: conf.DataPlane.Namespace,
	})

	if err != nil {
		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "Failed to get application from k8s %+v", err.Error()))
		return nil, "", err
	}

	return routeMetaList.Items, "", nil
}

// !!!TODO: Change this because now we have RoutePolicy CRs instead of AIProvider CRs.
// RetrieveAllAIProvidersFromK8s retrieves all the API CRs from the Kubernetes cluster
func RetrieveAllAIProvidersFromK8s(k8sClient client.Client, nextToken string) ([]dpv2alpha1.RoutePolicy, string, error) {
	conf, _ := config.ReadConfigs()
	aiProviderRPList := dpv2alpha1.RoutePolicyList{}
	err := k8sClient.List(context.Background(), &aiProviderRPList, &client.ListOptions{
		Namespace: conf.DataPlane.Namespace,
	})

	if err != nil {
		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "Failed to get ai provider route policies from k8s %+v", err.Error()))
		return nil, "", err
	}

	return aiProviderRPList.Items, "", nil
}

// !!!TODO: Change this becuase now we support this via BackendTrafficPolicy CRs.
// RetrieveAllRatelimitPoliciesSFromK8s retrieves all the API CRs from the Kubernetes cluster
func RetrieveAllRatelimitPoliciesSFromK8s(ratelimitName string, organization string, k8sClient client.Client) ([]gatewayv1alpha1.BackendTrafficPolicy, error) {
	conf, _ := config.ReadConfigs()
	rlBackendTPList := gatewayv1alpha1.BackendTrafficPolicyList{}
	resolvedRLBackendTPList := make([]gatewayv1alpha1.BackendTrafficPolicy, 0)
	labelMap := map[string]string{"kgw.wso2.com/policyName": ratelimitName, "kgw.wso2.com/organization": organization, "kgw.wso2.com/cpInitiated": "true"}
	// !!! Might need to change this later
	// Create a list option with the label selector
	listOption := &client.ListOptions{
		Namespace:     conf.DataPlane.Namespace,
		LabelSelector: labels.SelectorFromSet(labelMap),
	}
	var err error

	err = k8sClient.List(context.Background(), &rlBackendTPList, listOption)
	if err != nil {
		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "Failed to get ratelimitpolicies from k8s %v", err.Error()))
		return nil, err
	}
	resolvedRLBackendTPList = append(resolvedRLBackendTPList, rlBackendTPList.Items...)
	return resolvedRLBackendTPList, nil
}

// !!!TODO: Change this because now we support this via BackendTrafficPolicy + RoutePolicy CRs.
// RetrieveAllAIRatelimitPoliciesSFromK8s retrieves all the API CRs from the Kubernetes cluster
func RetrieveAllAIRatelimitPoliciesSFromK8s(aiRatelimitName string, organization string, k8sClient client.Client) ([]gatewayv1alpha1.BackendTrafficPolicy, error) {
	conf, _ := config.ReadConfigs()
	airlBackendTPList := gatewayv1alpha1.BackendTrafficPolicyList{}
	resolvedAIRLBackendTPList := make([]gatewayv1alpha1.BackendTrafficPolicy, 0)
	labelMap := map[string]string{"kgw.wso2.com/policyName": aiRatelimitName, "kgw.wso2.com/organization": organization, "kgw.wso2.com/cpInitiated": "true"}
	// !!! Might need to change this later
	// Create a list option with the label selector
	listOption := &client.ListOptions{
		Namespace:     conf.DataPlane.Namespace,
		LabelSelector: labels.SelectorFromSet(labelMap),
	}
	var err error

	err = k8sClient.List(context.Background(), &airlBackendTPList, listOption)
	if err != nil {
		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "Failed to get backend traffic policies for ai rate limiting from k8s %v", err.Error()))
		return nil, err
	}
	resolvedAIRLBackendTPList = append(resolvedAIRLBackendTPList, airlBackendTPList.Items...)
	return resolvedAIRLBackendTPList, nil
}

// PrepareSubscritionPolicyCRName prepare the cr name for a given policy name and organization pair
func PrepareSubscritionPolicyCRName(name, org string) string {
	return utils.GetSha1Value(fmt.Sprintf("%s-%s", name, org))
}

// getGatewayNameFromK8s gets the gateway name using the k8s client by selecting it using a label selector
func getGatewayNameFromK8s(k8sClient client.Client) (string, error) {
	conf, _ := config.ReadConfigs()
	gatewayList := gwapiv1.GatewayList{}
	// !!! TODO: Label name should be decided and added here.
	labelMap := map[string]string{"managed-by": "wso2kgw"}
	listOption := &client.ListOptions{
		Namespace:     conf.DataPlane.Namespace,
		LabelSelector: labels.SelectorFromSet(labelMap),
	}
	err := k8sClient.List(context.Background(), &gatewayList, listOption)
	if err != nil {
		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "Failed to get the required gateway from k8s %v", err.Error()))
		return "", err
	}
	if len(gatewayList.Items) == 0 {
		loggers.LoggerK8sClient.ErrorC(logging.PrintError(logging.Error1102, logging.CRITICAL, "No gateway found with the label selector %v", labelMap))
		return "", errors.New("no gateway found with the label selector")
	}
	return gatewayList.Items[0].Name, nil
}

// getRateLimitPolicyContents returns the time unit and count for the given policy based on the quota type
func getRateLimitPolicyContents(policy eventhubTypes.SubscriptionPolicy) (gatewayv1alpha1.RateLimitUnit, uint) {
	var timeUnit gatewayv1alpha1.RateLimitUnit
	var count, unitTime int
	switch policy.QuotaType {
	case "aiApiQuota":
		// For AI RL, the time unit is already properly formatted
		unitTime = int(policy.DefaultLimit.AiAPIQuota.UnitTime)
		loggers.LoggerK8sClient.Infof("Formatted Time Unit(AIAPIQuota): %s", policy.DefaultLimit.AiAPIQuota.TimeUnit)
		timeUnit = gatewayv1alpha1.RateLimitUnit(policy.DefaultLimit.AiAPIQuota.TimeUnit)
		count = int(*policy.DefaultLimit.AiAPIQuota.RequestCount) / unitTime
	case "eventCount":
		unitTime = int(policy.DefaultLimit.EventCount.UnitTime)
		loggers.LoggerK8sClient.Infof("Formatted Time Unit(EventCount): %s", getFormattedTimeUnit(policy.DefaultLimit.EventCount.TimeUnit))
		timeUnit = gatewayv1alpha1.RateLimitUnit(getFormattedTimeUnit(policy.DefaultLimit.EventCount.TimeUnit))
		count = int(policy.DefaultLimit.EventCount.EventCount) / unitTime
	case "requestCount":
		unitTime = int(policy.DefaultLimit.RequestCount.UnitTime)
		loggers.LoggerK8sClient.Infof("Formatted Time Unit(RequestCount): %s", getFormattedTimeUnit(policy.DefaultLimit.EventCount.TimeUnit))
		timeUnit = gatewayv1alpha1.RateLimitUnit(getFormattedTimeUnit(policy.DefaultLimit.RequestCount.TimeUnit))
		count = int(policy.DefaultLimit.RequestCount.RequestCount) / unitTime
	default:
		loggers.LoggerK8sClient.Errorf("Unexpected quota type %s", policy.QuotaType)
		return "", 0
	}
	return timeUnit, uint(count)
}

func getFormattedTimeUnit(timeUnit string) string {
	switch timeUnit {
	case "min":
		return "Minute"
	case "hours":
		return "Hour"
	case "days":
		return "Day"
	case "months":
		return "Day" // Changed this because the envoy CRD does not support month
	default:
		loggers.LoggerK8sClient.Errorf("Unexpected timeunit %s", timeUnit)
		return timeUnit
	}
}

// GenerateKMBackendCR create the backend for the Key Manager
func GenerateKMBackendCR(km eventhubTypes.ResolvedKeyManager, backendPort int, backendName, namespace, hostname string) *gatewayv1alpha1.Backend {
	if backendPort < 0 {
		loggers.LoggerK8sClient.Errorf("Invalid backend port provided: %d", backendPort)
		return nil
	}
	// Create Backend resource
	backend := &gatewayv1alpha1.Backend{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backendName,
			Namespace: namespace,
			Labels: map[string]string{
				"kgw.wso2.com/name":         km.Name,
				"kgw.wso2.com/organization": km.Organization,
				"kgw.wso2.com/cpInitiated":  "true",
			},
		},
		Spec: gatewayv1alpha1.BackendSpec{
			Endpoints: []gatewayv1alpha1.BackendEndpoint{
				{
					FQDN: &gatewayv1alpha1.FQDNEndpoint{
						Hostname: hostname,
						Port:     int32(backendPort),
					},
				},
			},
		},
	}
	return backend
}

// GenerateKMBackendTLSCR create the backend TLS policy for the Key Manager
func GenerateKMBackendTLSCR(km eventhubTypes.ResolvedKeyManager, backendName, namespace, hostname string) (*gwapiv1a3.BackendTLSPolicy, *corev1.Secret) {
	var validation gwapiv1a3.BackendTLSPolicyValidation
	var secret *corev1.Secret
	conf, _ := config.ReadConfigs()
	loggers.LoggerK8sClient.Infof("CA Cert Secret Name: %s", conf.ControlPlane.Certificates.CaCertSecretName)
	if km.KeyManagerConfig.CertificateType == "JWKS" {
		loggers.LoggerK8sClient.Info("JWKS Certificate Type")
		validation = gwapiv1a3.BackendTLSPolicyValidation{
			Hostname: v1.PreciseHostname(hostname),
			CACertificateRefs: []v1.LocalObjectReference{
				{
					Group: "",
					Kind:  "Secret",
					Name:  gwapiv1a2.ObjectName(conf.ControlPlane.Certificates.CaCertSecretName),
				},
			},
		}
	} else if km.KeyManagerConfig.CertificateType == "PEM" {
		loggers.LoggerK8sClient.Info("PEM Certificate Type")
		// Define the Secret object
		secret = &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf(backendName + "-cert-secret"),
				Namespace: namespace,
				Labels: map[string]string{
					"kgw.wso2.com/name":         km.Name,
					"kgw.wso2.com/organization": km.Organization,
					"kgw.wso2.com/cpInitiated":  "true",
				},
			},
			Type: corev1.SecretTypeOpaque, // Use Opaque for generic data
			Data: map[string][]byte{
				"certificateKey": []byte(km.KeyManagerConfig.CertificateValue),
			},
		}

		validation = gwapiv1a3.BackendTLSPolicyValidation{
			Hostname: v1.PreciseHostname(hostname),
			CACertificateRefs: []v1.LocalObjectReference{
				{
					Group: "",
					Kind:  "Secret",
					Name:  gwapiv1a2.ObjectName(backendName + "-cert-secret"),
				},
			},
		}
	}

	backendTLSPolicy := &gwapiv1a3.BackendTLSPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      backendName,
			Namespace: namespace,
			Labels: map[string]string{
				"kgw.wso2.com/name":         km.Name,
				"kgw.wso2.com/organization": km.Organization,
				"kgw.wso2.com/cpInitiated":  "true",
			},
		},
		Spec: gwapiv1a3.BackendTLSPolicySpec{
			TargetRefs: []gwapiv1a2.LocalPolicyTargetReferenceWithSectionName{
				{
					LocalPolicyTargetReference: gwapiv1a2.LocalPolicyTargetReference{
						Group: "gateway.envoyproxy.io",
						Kind:  "Backend",
						Name:  gwapiv1a2.ObjectName(backendName),
					},
				},
			},
			Validation: validation,
		},
	}
	return backendTLSPolicy, secret
}

// DeleteBackendCRByName removes a specific Backend CR by name
func DeleteBackendCRByName(backendName, namespace string, k8sClient client.Client) error {
	backend := &gatewayv1alpha1.Backend{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{Namespace: namespace, Name: backendName}, backend)
	if err != nil {
		if k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Infof("Backend CR '%s' not found in namespace '%s'", backendName, namespace)
			return nil // Not an error if already deleted
		}
		loggers.LoggerK8sClient.Errorf("Unable to get Backend CR '%s': %v", backendName, err)
		return err
	}
	err = k8sClient.Delete(context.Background(), backend, &client.DeleteOptions{})
	if err != nil {
		loggers.LoggerK8sClient.Errorf("Unable to delete Backend CR '%s': %v", backendName, err)
		return err
	}
	loggers.LoggerK8sClient.Infof("Successfully deleted Backend CR '%s'", backendName)
	return nil
}

// RetrieveSharedSubscriptionRateLimitPolicyFromK8s retrieves the shared BackendTrafficPolicy for the given organization
// and extracts all policy names from the rules headers
func RetrieveSharedSubscriptionRateLimitPolicyFromK8s(organization string, k8sClient client.Client) ([]string, error) {
	conf, _ := config.ReadConfigs()
	sharedPolicyName := utils.CreateSubscriptionPolicyName(SharedRateLimitPolicyName, organization)

	// Get the shared BackendTrafficPolicy
	existingPolicy := &gatewayv1alpha1.BackendTrafficPolicy{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: conf.DataPlane.Namespace,
		Name:      sharedPolicyName,
	}, existingPolicy)

	if err != nil {
		if k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Infof("Shared RateLimit BackendTrafficPolicy not found for organization: %s", organization)
			return []string{}, nil // Return empty slice, not an error
		}
		loggers.LoggerK8sClient.Errorf("Unable to get shared RateLimit BackendTrafficPolicy CR: %v", err)
		return nil, err
	}

	// Extract policy names from the rules headers
	policyNames := []string{}
	for _, rule := range existingPolicy.Spec.RateLimit.Global.Rules {
		for _, selector := range rule.ClientSelectors {
			for _, header := range selector.Headers {
				if header.Name == "policy-id" && header.Value != nil && *header.Value != "" {
					policyNames = append(policyNames, *header.Value)
					break // Found policy-id for this rule, break inner loops
				}
			}
		}
	}

	loggers.LoggerK8sClient.Debugf("Retrieved %d policy names from shared BackendTrafficPolicy for organization %s: %v",
		len(policyNames), organization, policyNames)
	return policyNames, nil
}

// DeploySubscriptionRateLimitPolicyCR applies the given RateLimitPolicies struct to the Kubernetes cluster.
func DeploySharedSubscriptionRateLimitPolicyCR(policy eventhubTypes.SubscriptionPolicy, k8sClient client.Client, aiPolicy bool) {
	conf, _ := config.ReadConfigs()

	unit, requestsPerUnit := getRateLimitPolicyContents(policy)
	loggers.LoggerK8sClient.Infof("Requests Per Unit after parsing: %d | Unit: %s", requestsPerUnit, unit)

	gatewayName, _ := getGatewayNameFromK8s(k8sClient)
	loggers.LoggerK8sClient.Infof("Gateway Name fetched from the k8s cluster: %s", gatewayName)
	if gatewayName == "" {
		gatewayName = "wso2-kgw-default"
	}
	sharedPolicyName := utils.CreateSubscriptionPolicyName(SharedRateLimitPolicyName, policy.TenantDomain)
	// Create the new rule for this policy
	var newRule gatewayv1alpha1.RateLimitRule
	if !aiPolicy {
		newRule = gatewayv1alpha1.RateLimitRule{
			ClientSelectors: []gatewayv1alpha1.RateLimitSelectCondition{
				{
					Headers: []gatewayv1alpha1.HeaderMatch{
						{
							Name:   "x-wso2-api-id",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "x-wso2-organization",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "x-wso2-subscription-id",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "policy-id",
							Value:  &policy.Name,
							Invert: ptr.To(false),
						},
					},
				},
			},
			Limit: gatewayv1alpha1.RateLimitValue{
				Requests: requestsPerUnit,
				Unit:     unit,
			},
			Shared: ptr.To(false),
		}
	} else {
		// Create the new rule for this AI policy
		newRule = gatewayv1alpha1.RateLimitRule{
			ClientSelectors: []gatewayv1alpha1.RateLimitSelectCondition{
				{
					Headers: []gatewayv1alpha1.HeaderMatch{
						{
							Name:   "x-wso2-api-id",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "x-wso2-organization",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "x-wso2-subscription-id",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "cost",
							Value:  ptr.To(""),
							Invert: ptr.To(false),
						},
						{
							Name:   "policy-id",
							Value:  &policy.Name,
							Invert: ptr.To(false),
						},
					},
				},
			},
			Limit: gatewayv1alpha1.RateLimitValue{
				Requests: requestsPerUnit,
				Unit:     unit,
			},
			Shared: ptr.To(false),
		}
	}

	// Try to get existing shared BackendTrafficPolicy
	existingPolicy := &gatewayv1alpha1.BackendTrafficPolicy{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: conf.DataPlane.Namespace,
		Name:      sharedPolicyName,
	}, existingPolicy)

	if err != nil {
		if k8error.IsNotFound(err) {
			// Create new shared BackendTrafficPolicy
			sharedPolicy := &gatewayv1alpha1.BackendTrafficPolicy{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sharedPolicyName,
					Namespace: conf.DataPlane.Namespace,
					Labels: map[string]string{
						"kgw.wso2.com/cpInitiated":  "true",
						"kgw.wso2.com/type":         "subscription-ratelimit",
						"kgw.wso2.com/organization": policy.TenantDomain,
					},
				},
				Spec: gatewayv1alpha1.BackendTrafficPolicySpec{
					MergeType: ptr.To(gatewayv1alpha1.MergeType("StrategicMerge")),
					RateLimit: &gatewayv1alpha1.RateLimitSpec{
						Type: gatewayv1alpha1.RateLimitType("Global"),
						Global: &gatewayv1alpha1.GlobalRateLimit{
							Rules: []gatewayv1alpha1.RateLimitRule{newRule},
						},
					},
					PolicyTargetReferences: gatewayv1alpha1.PolicyTargetReferences{
						TargetRefs: []gwapiv1a2.LocalPolicyTargetReferenceWithSectionName{
							{
								LocalPolicyTargetReference: gwapiv1a2.LocalPolicyTargetReference{
									Group: gwapiv1a2.Group(constants.GatewayGroup),
									Kind:  gwapiv1a2.Kind("Gateway"),
									Name:  gwapiv1a2.ObjectName(gatewayName),
								},
							},
						},
					},
				},
			}

			if err := k8sClient.Create(context.Background(), sharedPolicy); err != nil {
				loggers.LoggerK8sClient.Errorf("Unable to create the initial shared subscription rateLimit BackendTrafficPolicy CR: %+v", err)
			} else {
				loggers.LoggerK8sClient.Infof("Shared Subscription RateLimit BackendTrafficPolicy CR created: %s", sharedPolicy.Name)
			}
		} else {
			loggers.LoggerK8sClient.Errorf("Unable to get shared Subscription RateLimit BackendTrafficPolicy CR: %+v", err)
		}
	} else {
		// Update existing shared BackendTrafficPolicy by adding/updating the rule
		ruleUpdated := false

		// Check if a rule for this policy already exists
		for i, rule := range existingPolicy.Spec.RateLimit.Global.Rules {
			// Check if this rule belongs to the current policy by examining headers
			for _, selector := range rule.ClientSelectors {
				for _, header := range selector.Headers {
					if header.Name == "policy-id" && header.Value != nil && *header.Value == policy.Name {
						// Update existing rule
						existingPolicy.Spec.RateLimit.Global.Rules[i] = newRule
						ruleUpdated = true
						break
					}
				}
				if ruleUpdated {
					break
				}
			}
			if ruleUpdated {
				break
			}
		}

		// If no existing rule found, add new rule
		if !ruleUpdated {
			existingPolicy.Spec.RateLimit.Global.Rules = append(existingPolicy.Spec.RateLimit.Global.Rules, newRule)
		}

		if err := k8sClient.Update(context.Background(), existingPolicy); err != nil {
			loggers.LoggerK8sClient.Errorf("Unable to update shared RateLimit BackendTrafficPolicy CR: %+v", err)
		} else {
			loggers.LoggerK8sClient.Infof("Shared RateLimit BackendTrafficPolicy CR updated with policy: %s", policy.Name)
		}
	}
}

// UnDeploySubscriptionRateLimitPolicyCR removes the rate limit rule for the given policy from the shared BackendTrafficPolicy.
func UnDeploySharedSubscriptionRateLimitPolicyCR(policyName string, tenantDomain string, k8sClient client.Client, aiPolicy bool) {
	conf, _ := config.ReadConfigs()
	sharedPolicyName := utils.CreateSubscriptionPolicyName(SharedRateLimitPolicyName, tenantDomain)
	// Get the shared BackendTrafficPolicy
	existingPolicy := &gatewayv1alpha1.BackendTrafficPolicy{}
	err := k8sClient.Get(context.Background(), client.ObjectKey{
		Namespace: conf.DataPlane.Namespace,
		Name:      sharedPolicyName,
	}, existingPolicy)

	if err != nil {
		if k8error.IsNotFound(err) {
			loggers.LoggerK8sClient.Infof("Shared RateLimit BackendTrafficPolicy not found, nothing to remove for policy: %s", policyName)
		} else {
			loggers.LoggerK8sClient.Errorf("Unable to get shared RateLimit BackendTrafficPolicy CR: %v", err)
		}
		return
	}

	// Remove the rule for this policy
	updatedRules := []gatewayv1alpha1.RateLimitRule{}
	ruleFound := false

	for _, rule := range existingPolicy.Spec.RateLimit.Global.Rules {
		shouldKeepRule := true
		for _, selector := range rule.ClientSelectors {
			for _, header := range selector.Headers {
				if header.Name == "policy-id" && header.Value != nil && *header.Value == policyName {
					shouldKeepRule = false
					ruleFound = true
					break
				}
			}
			if !shouldKeepRule {
				break
			}
		}
		if shouldKeepRule {
			updatedRules = append(updatedRules, rule)
		}
	}

	if !ruleFound {
		loggers.LoggerK8sClient.Infof("Rule for policy '%s' not found in shared BackendTrafficPolicy", policyName)
		return
	}

	// Update or delete the policy based on remaining rules
	if len(updatedRules) == 0 {
		// No rules left, delete the entire BackendTrafficPolicy
		if err := k8sClient.Delete(context.Background(), existingPolicy); err != nil {
			loggers.LoggerK8sClient.Errorf("Unable to delete shared RateLimit BackendTrafficPolicy CR: %v", err)
		} else {
			loggers.LoggerK8sClient.Infof("Shared RateLimit BackendTrafficPolicy CR deleted as no rules remain")
		}
	} else {
		// Update with remaining rules
		existingPolicy.Spec.RateLimit.Global.Rules = updatedRules
		if err := k8sClient.Update(context.Background(), existingPolicy); err != nil {
			loggers.LoggerK8sClient.Errorf("Unable to update shared RateLimit BackendTrafficPolicy CR after removing rule: %v", err)
		} else {
			loggers.LoggerK8sClient.Infof("Rule for policy '%s' removed from shared BackendTrafficPolicy", policyName)
		}
	}
}
