package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/codec"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	claimtypes "github.com/tharsis/evmos/v2/x/claims/types"
)

type AppSate struct {
	Auth  authtypes.GenesisState
	Bank  banktypes.GenesisState
	Claim claimtypes.GenesisState
}

func getClaimGenesisState(genesis []byte, cdc codec.JSONCodec, extra bool) (AppSate, error) {
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

	if extra {
		var authGen authtypes.GenesisState
		err = cdc.UnmarshalJSON(appState[authtypes.ModuleName], &authGen)
		if err != nil {
			return app, err
		}
		app.Auth = authGen

		var bankGen banktypes.GenesisState
		err = cdc.UnmarshalJSON(appState[banktypes.ModuleName], &bankGen)
		if err != nil {
			return app, err
		}
		app.Bank = bankGen
	}

	app.Claim = claimGen

	return app, nil
}

func getBankBalance(records map[string]banktypes.Balance, address string) (sdk.Coins, error) {
	balance, ok := records[address]
	if !ok {
		return sdk.Coins{}, fmt.Errorf("balance %s not found", address)
	}

	return balance.Coins, nil
}

func convertExpGenToMap(records []claimtypes.ClaimsRecordAddress) map[string]claimtypes.ClaimsRecordAddress {
	result := make(map[string]claimtypes.ClaimsRecordAddress)
	sumUnclaimed := sdk.ZeroInt()
	actions := sdk.NewInt(4)
	for _, record := range records {
		result[record.Address] = record
		initialClaimablePerAction := record.InitialClaimableAmount.Quo(actions)
		for _, actionCompleted := range record.ActionsCompleted {
			if !actionCompleted {
				// NOTE: only add the initial claimable amount per action for the ones that haven't been claimed
				sumUnclaimed = sumUnclaimed.Add(initialClaimablePerAction)
			}
		}
	}

	fmt.Printf("Total unclaimed: %s\n", sumUnclaimed.String())

	return result
}

func convertBalancesToMap(balances []banktypes.Balance) map[string]banktypes.Balance {
	result := make(map[string]banktypes.Balance)
	for _, balance := range balances {
		result[balance.Address] = balance
	}

	return result
}

func convertAccountsToMap(authGen authtypes.GenesisState, cdc codec.Codec) (map[string]authtypes.AccountI, error) {
	result := make(map[string]authtypes.AccountI)
	for _, account := range authGen.Accounts {
		var ethAccount authtypes.AccountI
		err := cdc.UnpackAny(account, &ethAccount)
		if err != nil {
			return result, err
		}

		result[ethAccount.GetAddress().String()] = ethAccount
	}

	return result, nil
}

func printDiffs(gen, exp AppSate, cdc codec.Codec) error {
	// minAmount := sdk.NewInt(1000000000000000)
	diffs := make([]claimtypes.ClaimsRecordAddress, 0)
	totalBalance := sdk.ZeroInt()
	total50Percent := sdk.ZeroInt()
	initalAmount := sdk.ZeroInt()

	expMap := convertExpGenToMap(exp.Claim.ClaimsRecords)
	expBanks := convertBalancesToMap(exp.Bank.Balances)
	expAccounts, err := convertAccountsToMap(exp.Auth, cdc)
	if err != nil {
		return err
	}

	empData := [][]string{
		{"Account", "Balance", "InitialClaim", "Sequence", "TotalCoins"},
	}

	for _, genRecord := range gen.Claim.ClaimsRecords {
		initalAmount = initalAmount.Add(genRecord.InitialClaimableAmount)
		_, ok := expMap[genRecord.Address]
		if ok {
			continue
		}

		acc, ok := expAccounts[genRecord.Address]
		if !ok {
			return fmt.Errorf("account %s not found", genRecord.Address)
		}

		// no locked account
		if acc.GetSequence() != 0 {
			continue
		}

		// record not found in exported
		balances, err := getBankBalance(expBanks, genRecord.Address)
		if err != nil {
			return err
		}

		amount50percent := genRecord.InitialClaimableAmount.Quo(sdk.NewInt(2))
		evmosBalance := balances.AmountOf("aevmos")
		if evmosBalance.GTE(amount50percent) {
			diffs = append(diffs, genRecord)

			totalBalance = totalBalance.Add(evmosBalance)
			total50Percent = total50Percent.Add(amount50percent)

			empData = append(empData, []string{
				genRecord.Address, evmosBalance.String(), genRecord.InitialClaimableAmount.String(),
				strconv.Itoa(int(acc.GetSequence())), strconv.Itoa(balances.Len()),
			})
		}
	}

	fmt.Printf("Total initial amount: %s\n", initalAmount.String())
	fmt.Printf("Total missed accounts: %d\n", len(diffs))
	fmt.Printf("Total balance (missed accounts): %s\n", totalBalance.String())
	fmt.Printf("Total missed claimable 50%%: %s\n", total50Percent.String())

	// saved to csv
	csvFile, err := os.Create("claims.csv")

	if err != nil {
		return err
	}
	csvwriter := csv.NewWriter(csvFile)

	for _, empRow := range empData {
		_ = csvwriter.Write(empRow)
	}

	csvwriter.Flush()
	csvFile.Close()

	return nil
}

func CheckClaimsCmd(cdc codec.Codec) *cobra.Command {
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
			defer expGenesisJSON.Close()

			genBytes, err := ioutil.ReadAll(genesisJSON)
			if err != nil {
				return err
			}

			expGenBytes, err := ioutil.ReadAll(expGenesisJSON)
			if err != nil {
				return err
			}

			claimsGen, err := getClaimGenesisState(genBytes, cdc, false)
			if err != nil {
				return err
			}

			claimsExp, err := getClaimGenesisState(expGenBytes, cdc, true)
			if err != nil {
				return err
			}
			fmt.Printf("Genesis: %d recods\n", len(claimsGen.Claim.ClaimsRecords))
			fmt.Printf("Exported: %d recods\n", len(claimsExp.Claim.ClaimsRecords))

			return printDiffs(claimsGen, claimsExp, cdc)
		},
	}

	return cmd
}
