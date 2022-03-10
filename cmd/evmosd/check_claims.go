package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	claimtypes "github.com/tharsis/evmos/v2/x/claims/types"
)

type AppSate struct {
	Bank  banktypes.GenesisState
	Claim claimtypes.GenesisState
}

func getClaimGenesisState(genesis []byte, cdc codec.JSONCodec) (AppSate, error) {
	var gen map[string]json.RawMessage
	var app AppSate

	err := json.Unmarshal(genesis, &gen)
	if err != nil {
		return app, err
	}

	var appState map[string]json.RawMessage
	err = json.Unmarshal(gen["app_state"], &appState)
	if err != nil {
		return app, err
	}

	var claimGen claimtypes.GenesisState
	err = cdc.UnmarshalJSON(appState[claimtypes.ModuleName], &claimGen)
	if err != nil {
		return app, err
	}

	var bankGen banktypes.GenesisState
	err = cdc.UnmarshalJSON(appState[banktypes.ModuleName], &bankGen)
	if err != nil {
		return app, err
	}

	app.Claim = claimGen
	app.Bank = bankGen

	return app, nil
}

func ExistInRecords(records []claimtypes.ClaimsRecordAddress, address string) bool {
	for _, record := range records {
		if record.Address == address {
			return true
		}
	}

	return false
}

func getBankBalance(records map[string]banktypes.Balance, address, denom string) sdk.Int {
	balance, ok := records[address]
	if !ok {
		return sdk.ZeroInt()
	}

	return balance.Coins.AmountOf(denom)
}

func convertToMap(records []claimtypes.ClaimsRecordAddress) map[string]claimtypes.ClaimsRecordAddress {
	result := make(map[string]claimtypes.ClaimsRecordAddress)
	initalAmount := sdk.ZeroInt()
	for _, record := range records {
		result[record.Address] = record
		initalAmount = initalAmount.Add(record.InitialClaimableAmount)
	}

	fmt.Printf("Total initial amount: %s\n", initalAmount.String())
	return result
}

func convertBalancesToMap(balances []banktypes.Balance) map[string]banktypes.Balance {
	result := make(map[string]banktypes.Balance)
	for _, balance := range balances {
		result[balance.Address] = balance
	}

	return result
}

func printDiffs(gen, exp AppSate) error {
	minAmount := sdk.NewInt(1000000000000000)
	diffs := make([]claimtypes.ClaimsRecordAddress, 0)
	totalBalance := sdk.ZeroInt()
	total50Percent := sdk.ZeroInt()

	expMap := convertToMap(exp.Claim.ClaimsRecords)
	expBanks := convertBalancesToMap(exp.Bank.Balances)

	for _, genRecord := range gen.Claim.ClaimsRecords {
		_, ok := expMap[genRecord.Address]
		if !ok {
			continue
		}

		balance := getBankBalance(expBanks, genRecord.Address, "aevmos")
		if balance.GT(minAmount) {
			diffs = append(diffs, genRecord)

			amount50percent := genRecord.InitialClaimableAmount.Quo(sdk.NewInt(2))
			totalBalance = totalBalance.Add(balance)
			total50Percent = total50Percent.Add(amount50percent)
		}
	}

	fmt.Printf("Total missed accounts: %d\n", len(diffs))
	fmt.Printf("Total missed balance: %s\n", totalBalance.String())
	fmt.Printf("Total missed 50%%: %s\n", total50Percent.String())

	return nil
}

func CheckClaimsCmd(cdc codec.JSONCodec) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-claims [input-genesis-file] [exported-genesis-file]",
		Short: "Verify claims records",
		Long: `Verify claims records and balanceas
Example:
    evmosd check-claims genesis.json
`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			genesisFile := args[0]
			expGenesisFile := args[1]

			genesisJSON, err := os.Open(genesisFile)
			if err != nil {
				return err
			}
			defer genesisJSON.Close()

			expGenesisJSON, err := os.Open(expGenesisFile)
			if err != nil {
				return err
			}
			defer genesisJSON.Close()

			genBytes, err := ioutil.ReadAll(genesisJSON)
			if err != nil {
				return err
			}

			expGenBytes, err := ioutil.ReadAll(expGenesisJSON)
			if err != nil {
				return err
			}

			claimsGen, err := getClaimGenesisState(genBytes, cdc)
			if err != nil {
				return err
			}

			claimsExp, err := getClaimGenesisState(expGenBytes, cdc)
			if err != nil {
				return err
			}
			fmt.Printf("Genesis: %d recods\n", len(claimsGen.Claim.ClaimsRecords))
			fmt.Printf("Exported: %d recods\n", len(claimsExp.Claim.ClaimsRecords))

			return printDiffs(claimsGen, claimsExp)
		},
	}

	return cmd
}
