package exchange

import (
	"encoding/json"
	"fmt"
	"github.com/open-horizon/anax/cli/cliconfig"
	"github.com/open-horizon/anax/cli/cliutils"
	"github.com/open-horizon/anax/cutil"
	"github.com/open-horizon/anax/exchange"
	"github.com/open-horizon/anax/exchangecommon"
	"github.com/open-horizon/anax/i18n"
	"net/http"
)

func NMPList(org, credToUse, nmpName string, namesOnly, listNodes bool) {
	// if user specifies --nodes flag, return list of applicable nodes for given nmp instead of the nmp itself.
	if listNodes {
		NMPListNodes(org, credToUse, nmpName)
		return
	}

	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	if nmpName == "*" {
		nmpName = ""
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var nmpList exchange.ExchangeNodeManagementPolicyResponse
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nmpList)
	if httpCode == 404 && nmpName != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("NMP %s not found in org %s", nmpName, nmpOrg))
	} else if httpCode == 404 {
		policyNameList := []string{}
		fmt.Println(policyNameList)
	} else if namesOnly && nmpName == "" {
		nmpNameList := []string{}
		for nmp := range nmpList.Policies {
			nmpNameList = append(nmpNameList, nmp)
		}
		jsonBytes, err := json.MarshalIndent(nmpNameList, "", cliutils.JSON_INDENT)
		if err != nil {
			cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to marshal 'hzn exchange nmp list' output: %v", err))
		}
		fmt.Println(string(jsonBytes))
	} else {
		output := cliutils.MarshalIndent(nmpList.Policies, "exchange nmp list")
		fmt.Println(output)
	}
}

func NMPNew() {
	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	var nmp_template = []string{
		`{`,
		`  "label": "",                               /* ` + msgPrinter.Sprintf("A short description of the policy.") + ` */`,
		`  "description": "",                         /* ` + msgPrinter.Sprintf("(Optional) A much longer description of the policy.") + ` */`,
		`  "constraints": [                           /* ` + msgPrinter.Sprintf("(Optional) A list of constraint expressions of the form <property name> <operator> <property value>,") + ` */`,
		`    "myproperty == myvalue"                  /* ` + msgPrinter.Sprintf("separated by boolean operators AND (&&) or OR (||).") + `*/`,
		`  ],`,
		`  "properties": [                            /* ` + msgPrinter.Sprintf("(Optional) A list of policy properties that describe this policy.") + ` */`,
		`    {`,
		`      "name": "",`,
		`      "value": null`,
		`    }`,
		`  ],`,
		`  "patterns": [                              /* ` + msgPrinter.Sprintf("(Optional) This policy applies to nodes using one of these patterns.") + ` */`,
		`    ""`,
		`  ],`,
		`  "enabled": false,                          /* ` + msgPrinter.Sprintf("Is this policy enabled or disabled.") + ` */`,
		`  "start": "<RFC3339 timestamp> | now",      /* ` + msgPrinter.Sprintf("When to start an upgrade, default \"now\".") + ` */`,
		`  "startWindow": 0,                          /* ` + msgPrinter.Sprintf("Enable agents to randomize upgrade start time within start + startWindow, default 0.") + ` */`,
		`  "agentUpgradePolicy": {                    /* ` + msgPrinter.Sprintf("(Optional) Assertions on how the agent should update itself.") + ` */`,
		`    "manifest": "",                          /* ` + msgPrinter.Sprintf("The manifest file containing the software, config and cert files to upgrade.") + ` */`,
		`    "allowDowngrade": false                  /* ` + msgPrinter.Sprintf("Is this policy allowed to perform a downgrade to a previous version.") + ` */`,
		`  }`,
		`}`,
	}

	for _, s := range nmp_template {
		fmt.Println(s)
	}
}

func NMPAdd(org, credToUse, nmpName, jsonFilePath string, appliesTo, noConstraints bool) {
	// check for ExchangeUrl early on
	var exchUrl = cliutils.GetExchangeUrl()

	cliutils.SetWhetherUsingApiKey(credToUse)
	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// read in the new nmp from file
	newBytes := cliconfig.ReadJsonFileWithLocalConfig(jsonFilePath)
	var nmpFile exchangecommon.ExchangeNodeManagementPolicy
	err := json.Unmarshal(newBytes, &nmpFile)
	if err != nil {
		cliutils.Fatal(cliutils.JSON_PARSING_ERROR, msgPrinter.Sprintf("failed to unmarshal json input file %s: %v", jsonFilePath, err))
	}

	// validate the format of the nmp
	err = nmpFile.Validate()
	if err != nil {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Incorrect node management policy format in file %s: %v", jsonFilePath, err))
	}

	// if the --no-constraints flag is not specified and the given nmp has no constraints, alert the user.
	if !noConstraints && nmpFile.HasNoConstraints() && nmpFile.HasNoPatterns() {
		cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("The node management policy has no constraints which might result in the management policy being deployed to all nodes. Please specify --no-constraints to confirm that this is acceptable."))
	}

	var resp struct {
		Code string `json:"code"`
		Msg  string `json:"msg"`
	}
	// add/overwrite nmp file
	httpCode := cliutils.ExchangePutPost("Exchange", http.MethodPost, exchUrl, "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{201, 403}, nmpFile, &resp)
	if httpCode == 403 {
		//try to update the existing policy
		httpCode = cliutils.ExchangePutPost("Exchange", http.MethodPut, exchUrl, "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{201, 404}, nmpFile, nil)
		if httpCode == 201 {
			msgPrinter.Printf("Node management policy: %v/%v updated in the Horizon Exchange", nmpOrg, nmpName)
			msgPrinter.Println()
		} else if httpCode == 404 {
			cliutils.Fatal(cliutils.CLI_INPUT_ERROR, msgPrinter.Sprintf("Cannot create node management policy %v/%v: %v", nmpOrg, nmpName, resp.Msg))
		}
	} else if !cliutils.IsDryRun() {
		msgPrinter.Printf("Node management policy: %v/%v added in the Horizon Exchange", nmpOrg, nmpName)
		msgPrinter.Println()
	}
	if appliesTo {
		nodes := determineCompatibleNodes(org, credToUse, nmpName, nmpFile)
		if nodes != nil && len(nodes) > 0 {
			output := cliutils.MarshalIndent(nodes, "exchange nmp add")
			fmt.Printf(output)
		}
		msgPrinter.Println()
	}
}

func NMPRemove(org, credToUse, nmpName string, force bool) {
	cliutils.SetWhetherUsingApiKey(credToUse)
	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	if !force {
		cliutils.ConfirmRemove(msgPrinter.Sprintf("Are you sure you want to remove node management policy %v for org %v from the Horizon Exchange?", nmpName, nmpOrg))
	}

	//remove policy
	httpCode := cliutils.ExchangeDelete("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{204, 404})
	if httpCode == 404 {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("Node management policy %s not found in org %s", nmpName, nmpOrg))
	} else if httpCode == 204 {
		msgPrinter.Printf("Removing node management policy %v/%v from the exchange.", nmpOrg, nmpName)
		msgPrinter.Println()
		msgPrinter.Printf("Node management policy %v/%v removed", nmpOrg, nmpName)
		msgPrinter.Println()
	}
}

func NMPListNodes(org, credToUse, nmpName string) {
	cliutils.SetWhetherUsingApiKey(credToUse)

	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	if nmpName == "*" {
		nmpName = ""
	}

	// get message printer
	msgPrinter := i18n.GetMessagePrinter()

	// store list of compatible nodes in map indexed by NMP's in the exchange
	compatibleNodeMap := make(map[string][]string)

	var nmpList exchange.ExchangeNodeManagementPolicyResponse
	var output string
	httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nmpList)
	if httpCode == 404 && nmpName != "" {
		cliutils.Fatal(cliutils.NOT_FOUND, msgPrinter.Sprintf("NMP %s not found in org %s", nmpName, nmpOrg))
	} else if httpCode == 404 {
		output = "{}"
	} else {
		for nmp, nmpPolicy := range nmpList.Policies {
			nodes := determineCompatibleNodes(org, credToUse, nmpName, nmpPolicy)
			compatibleNodeMap[nmp] = nodes
		}
		output = cliutils.MarshalIndent(compatibleNodeMap, "management nmp list --nodes")
	}
	fmt.Printf(output)
	msgPrinter.Println()
}

func NMPStatus(org, credToUse, nmpName string) {
	// cliutils.SetWhetherUsingApiKey(credToUse)

	// var nmpOrg string
	// nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// if nmpName == "*" {
	// 	nmpName = ""
	// }

	// get message printer
	// msgPrinter := i18n.GetMessagePrinter()

	// var nmpList exchange.ExchangeNodeManagementPolicyResponse
	// httpCode := cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/managementpolicies"+cliutils.AddSlash(nmpName), cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nmpList)
}

func determineCompatibleNodes(org, credToUse, nmpName string, nmpPolicy exchangecommon.ExchangeNodeManagementPolicy) []string {
	var nmpOrg string
	nmpOrg, nmpName = cliutils.TrimOrg(org, nmpName)

	// get node(s) name(s) from the Exchange
	var resp ExchangeNodes
	cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/nodes", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &resp)

	compatibleNodes := []string{}
	for nodeNameEx, node := range resp.Nodes {
		// Only check registered nodes
		if node.PublicKey == "" {
			continue
		}
		if node.Pattern != "" {
			if cutil.SliceContains(nmpPolicy.Patterns, node.Pattern) {
				compatibleNodes = append(compatibleNodes, nodeNameEx)
			}
		} else {
			// list policy
			var nodePolicy exchange.ExchangeNodePolicy
			_, nodeName := cliutils.TrimOrg(org, nodeNameEx)
			cliutils.ExchangeGet("Exchange", cliutils.GetExchangeUrl(), "orgs/"+nmpOrg+"/nodes"+cliutils.AddSlash(nodeName)+"/policy", cliutils.OrgAndCreds(org, credToUse), []int{200, 404}, &nodePolicy)
			nodeManagementPolicy := nodePolicy.GetManagementPolicy()

			if err := nmpPolicy.Constraints.IsSatisfiedBy(nodeManagementPolicy.Properties); err != nil {
				continue
			} else if err = nodeManagementPolicy.Constraints.IsSatisfiedBy(nmpPolicy.Properties); err != nil {
				continue
			} else {
				compatibleNodes = append(compatibleNodes, nodeNameEx)
			}
		}
	}
	return compatibleNodes
}
