package terraformtests

import (
	"path"

	. "csbbrokerpakazure/terraform-tests/helpers"

	tfjson "github.com/hashicorp/terraform-json"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
)

var _ = Describe("CosmosDB SQL", Label("cosmosdb-sql-terraform"), Ordered, func() {
	const (
		instanceName      = "csb-cosmosdb-sql"
		resourceGroupName = "csb-resource-group"
		dbName            = "csb-db"
	)

	var (
		plan                  tfjson.Plan
		terraformProvisionDir string
	)

	defaultVars := map[string]any{
		"azure_client_id":                 azureClientID,
		"azure_client_secret":             azureClientSecret,
		"azure_subscription_id":           azureSubscriptionID,
		"azure_tenant_id":                 azureTenantID,
		"request_units":                   10000,
		"instance_name":                   instanceName,
		"failover_locations":              []string{"westus", "eastus"},
		"enable_multiple_write_locations": true,
		"enable_automatic_failover":       true,
		"resource_group":                  resourceGroupName,
		"db_name":                         dbName,
		"location":                        "westus",
		"ip_range_filter":                 "0.0.0.0",
		"consistency_level":               "BoundedStaleness",
		"max_interval_in_seconds":         5,
		"max_staleness_prefix":            100,
		"skip_provider_registration":      false,
		"authorized_network":              "",
		"private_dns_zone_ids":            []string{},
		"private_endpoint_subnet_id":      "",
		"labels":                          map[string]any{"k1": "v1"},
	}

	BeforeAll(func() {
		terraformProvisionDir = path.Join(workingDir, "azure-cosmosdb")
		Init(terraformProvisionDir)
	})

	Context("with Default values", func() {
		BeforeAll(func() {
			plan = ShowPlan(terraformProvisionDir, buildVars(defaultVars, map[string]any{}))
		})

		It("should create the right resources", func() {
			Expect(plan.ResourceChanges).To(HaveLen(2))

			Expect(ResourceChangesTypes(plan)).To(ConsistOf(
				"azurerm_cosmosdb_account",
				"azurerm_cosmosdb_sql_database",
			))
		})

		It("should create a cosmosdb account with the right values", func() {
			Expect(AfterValuesForType(plan, "azurerm_cosmosdb_account")).To(
				MatchKeys(IgnoreExtras, Keys{
					"name":                              Equal(instanceName),
					"location":                          Equal("westus"),
					"resource_group_name":               Equal(resourceGroupName),
					"offer_type":                        Equal("Standard"),
					"kind":                              Equal("GlobalDocumentDB"),
					"enable_automatic_failover":         BeTrue(),
					"enable_multiple_write_locations":   BeTrue(),
					"is_virtual_network_filter_enabled": BeFalse(),
					"ip_range_filter":                   Equal("0.0.0.0"),
					"tags": MatchAllKeys(Keys{
						"k1": Equal("v1"),
					}),

					"consistency_policy": ConsistOf(
						MatchKeys(IgnoreExtras, Keys{
							"consistency_level":       Equal("BoundedStaleness"),
							"max_interval_in_seconds": BeNumerically("==", 5),
							"max_staleness_prefix":    BeNumerically("==", 100),
						}),
					),

					"geo_location": ConsistOf(
						MatchKeys(IgnoreExtras, Keys{
							"location":          Equal("westus"),
							"failover_priority": BeNumerically("==", 0),
						}),
						MatchKeys(IgnoreExtras, Keys{
							"location":          Equal("eastus"),
							"failover_priority": BeNumerically("==", 1),
						}),
					),
				}),
			)
		})

		It("should create a cosmosdb sql database with the right values", func() {
			Expect(AfterValuesForType(plan, "azurerm_cosmosdb_sql_database")).To(
				MatchKeys(IgnoreExtras, Keys{
					"name":                Equal(dbName),
					"resource_group_name": Equal(resourceGroupName),
					"account_name":        Equal(instanceName),
					"throughput":          BeNumerically("==", 10000),
				}))
		})
	})

	When("no resource group is passed", func() {
		BeforeEach(func() {
			plan = ShowPlan(terraformProvisionDir, buildVars(defaultVars, map[string]any{
				"resource_group": "",
			}))
		})

		It("should create a resource group", func() {
			Expect(plan.ResourceChanges).To(HaveLen(3))

			Expect(ResourceChangesTypes(plan)).To(ConsistOf(
				"azurerm_resource_group",
				"azurerm_cosmosdb_account",
				"azurerm_cosmosdb_sql_database",
			))

			Expect(AfterValuesForType(plan, "azurerm_resource_group")).To(
				MatchKeys(IgnoreExtras, Keys{
					"name": Equal("rg-csb-cosmosdb-sql"),
				}))
		})
	})

	When("private endpoint is enabled", func() {
		var subnetID = "/subscriptions/azureSubscriptionID/resourceGroups/csb-cosmos-rg/providers/Microsoft.Network/virtualNetworks/csb-cosmos-rg-platform/subnets/csb-cosmos-rg-pas-subnet"
		var dnsID = "/subscriptions/azureSubscriptionID/resourceGroups/dns-configuration/providers/Microsoft.Network/privateDnsZones/test"

		BeforeEach(func() {
			plan = ShowPlan(terraformProvisionDir, buildVars(defaultVars, map[string]any{
				"private_endpoint_subnet_id": subnetID,
				"private_dns_zone_ids":       []string{dnsID},
			}))
		})

		It("should create a private endpoint", func() {
			Expect(plan.ResourceChanges).To(HaveLen(3))

			Expect(ResourceChangesTypes(plan)).To(ConsistOf(
				"azurerm_cosmosdb_account",
				"azurerm_cosmosdb_sql_database",
				"azurerm_private_endpoint",
			))

			Expect(AfterValuesForType(plan, "azurerm_private_endpoint")).To(
				MatchKeys(IgnoreExtras, Keys{
					"name":                Equal("csb-cosmosdb-sql-private_endpoint"),
					"location":            Equal("westus"),
					"resource_group_name": Equal(resourceGroupName),
					"subnet_id":           Equal(subnetID),
					"tags": MatchAllKeys(Keys{
						"k1": Equal("v1"),
					}),
					"private_service_connection": ConsistOf(MatchKeys(IgnoreExtras, Keys{
						"name":                 Equal("csb-cosmosdb-sql-private_service_connection"),
						"is_manual_connection": BeFalse(),
						"subresource_names":    ConsistOf("SQL"),
					})),
					"private_dns_zone_group": ConsistOf(MatchKeys(IgnoreExtras, Keys{
						"private_dns_zone_ids": ConsistOf(dnsID),
					})),
				}),
			)
		})
	})
})
