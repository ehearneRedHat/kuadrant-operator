package gatewayapi

import (
	"encoding/json"

	"sigs.k8s.io/controller-runtime/pkg/client"
	gatewayapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/kuadrant/kuadrant-operator/pkg/library/utils"
)

type TopologyIndexes struct {
	// gatewayPolicies is an index of gateways mapping to Kuadrant Policies which
	// directly or indirectly are targeting the indexed gateway.
	// When a kuadrant policy directly or indirectly targets a gateway, the policy's configuration
	// needs to be added to that gateway.
	// Type: Gateway -> []Policy
	gatewayPolicies map[client.ObjectKey][]GatewayAPIPolicy

	// policyRoute is an index of policies mapping to HTTPRoutes
	// The index only includes policies targeting only existing and accepted (by parent gateways) HTTPRoutes
	// Type: Policy -> HTTPRoute
	policyRoute map[client.ObjectKey]*gatewayapiv1.HTTPRoute

	// untargetedRoutes is an index of gateways mapping to HTTPRoutes not targeted by a kuadrant policy
	// Gateway -> []HTTPRoute
	untargetedRoutes map[client.ObjectKey][]*gatewayapiv1.HTTPRoute

	// Raw topology with gateways, routes and policies
	// Currently only used for logging
	internalTopology *GatewayAPITopology
}

func NewTopologyIndexes(t *GatewayAPITopology) *TopologyIndexes {
	if t == nil {
		return nil
	}

	return &TopologyIndexes{
		gatewayPolicies:  buildGatewayPoliciesIndex(t),
		policyRoute:      buildPolicyRouteIndex(t),
		untargetedRoutes: buildUntargetedRoutesIndex(t),
		internalTopology: t,
	}
}

// PoliciesFromGateway returns Kuadrant Policies which
// directly or indirectly are targeting the gateway given as input.
// Type: Gateway -> []Policy
func (k *TopologyIndexes) PoliciesFromGateway(gateway *gatewayapiv1.Gateway) []GatewayAPIPolicy {
	return k.gatewayPolicies[client.ObjectKeyFromObject(gateway)]
}

// GetPolicyHTTPRoute returns the HTTPRoute being targetd by the policy.
// The method only returns existing and accepted (by parent gateways) HTTPRoutes
// Type: Policy -> HTTPRoute
func (k *TopologyIndexes) GetPolicyHTTPRoute(policy GatewayAPIPolicy) *gatewayapiv1.HTTPRoute {
	return k.policyRoute[client.ObjectKeyFromObject(policy)]
}

// GetUntargetedRoutes returns the HTTPRoutes not targeted by any kuadrant policy
// having the gateway given as input as parent.
// Gateway -> []HTTPRoute
func (k *TopologyIndexes) GetUntargetedRoutes(gateway *gatewayapiv1.Gateway) []*gatewayapiv1.HTTPRoute {
	return k.untargetedRoutes[client.ObjectKeyFromObject(gateway)]
}

// String representation of the topology
// This is not designed to be a serialization format that could be deserialized
func (k *TopologyIndexes) String() string {
	policiesPerGateway := func() map[string][]string {
		index := make(map[string][]string, 0)
		for gatewayKey, policyList := range k.gatewayPolicies {
			index[gatewayKey.String()] = utils.Map(policyList, func(p GatewayAPIPolicy) string {
				return client.ObjectKeyFromObject(p).String()
			})
		}
		if len(index) == 0 {
			return nil
		}
		return index
	}()

	policiesTargetingRoutes := func() map[string]string {
		index := make(map[string]string, 0)
		for policyKey, route := range k.policyRoute {
			index[policyKey.String()] = client.ObjectKeyFromObject(route).String()
		}
		if len(index) == 0 {
			return nil
		}
		return index
	}()

	untargetedRoutesPerGateway := func() map[string][]string {
		index := make(map[string][]string, 0)
		for gatewayKey, routeList := range k.untargetedRoutes {
			index[gatewayKey.String()] = utils.Map(routeList, func(route *gatewayapiv1.HTTPRoute) string {
				return client.ObjectKeyFromObject(route).String()
			})
		}
		if len(index) == 0 {
			return nil
		}
		return index
	}()

	indexesRepr := struct {
		GatewayPolicies  map[string][]string `json:"policiesPerGateway"`
		PolicyRoute      map[string]string   `json:"policiesTargetingRoutes"`
		UntargetedRoutes map[string][]string `json:"untargetedRoutesPerGateway"`
	}{
		policiesPerGateway,
		policiesTargetingRoutes,
		untargetedRoutesPerGateway,
	}

	jsonData, err := json.MarshalIndent(indexesRepr, "", "  ")
	if err != nil {
		panic(err)
	}
	return string(jsonData)
}

func buildGatewayPoliciesIndex(t *GatewayAPITopology) map[client.ObjectKey][]GatewayAPIPolicy {
	// Build Gateway -> []Policy index with all the policies affecting the indexed gateway
	index := make(map[client.ObjectKey][]GatewayAPIPolicy, 0)
	for _, gatewayNode := range t.Gateways() {
		// Consisting of:
		// - Policy targeting directly the gateway
		// - Policies targeting the descendant routes of the gateway
		policies := make([]GatewayAPIPolicy, 0)

		policies = append(policies, gatewayNode.AttachedPolicies()...)

		for _, routeNode := range gatewayNode.Routes() {
			policies = append(policies, routeNode.AttachedPolicies()...)
		}

		index[gatewayNode.ObjectKey()] = policies
	}

	return index
}

func buildPolicyRouteIndex(t *GatewayAPITopology) map[client.ObjectKey]*gatewayapiv1.HTTPRoute {
	// Build Policy -> HTTPRoute index with the route targeted by the indexed policy
	index := make(map[client.ObjectKey]*gatewayapiv1.HTTPRoute, 0)
	for _, routeNode := range t.Routes() {
		for _, policy := range routeNode.AttachedPolicies() {
			index[client.ObjectKeyFromObject(policy)] = routeNode.Route()
		}
	}

	return index
}

func buildUntargetedRoutesIndex(t *GatewayAPITopology) map[client.ObjectKey][]*gatewayapiv1.HTTPRoute {
	// Build Gateway -> []HTTPRoute index with all the routes not targeted by a policy
	index := make(map[client.ObjectKey][]*gatewayapiv1.HTTPRoute, 0)

	for _, gatewayNode := range t.Gateways() {
		routes := make([]*gatewayapiv1.HTTPRoute, 0)

		for _, routeNode := range gatewayNode.Routes() {
			if len(routeNode.AttachedPolicies()) == 0 {
				routes = append(routes, routeNode.Route())
			}
		}

		index[gatewayNode.ObjectKey()] = routes
	}

	return index
}
