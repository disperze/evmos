package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	servertypes "github.com/cosmos/cosmos-sdk/server/types"
	claimtypes "github.com/tharsis/evmos/v2/x/claims/types"
)

func getClaimGenesisState(genesis []byte) (claimtypes.GenesisState, error) {
	var gen servertypes.ExportedApp
	var claimGen claimtypes.GenesisState

	err := json.Unmarshal(genesis, &gen)
	if err != nil {
		return claimGen, err
	}

	var appState map[string]json.RawMessage
	err = json.Unmarshal(gen.AppState, &appState)
	if err != nil {
		return claimGen, err
	}

	err = json.Unmarshal(appState[claimtypes.ModuleName], &claimGen)
	if err != nil {
		return claimGen, err
	}

	return claimGen, nil
}

func CheckClaimsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "check-claims [input-genesis-file]",
		Short: "Verify claims records",
		Long: `Verify claims records and balanceas
Example:
    evmosd check-claims genesis.json
`,
		Args: cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			genesisFile := args[0]

			genesisJSON, err := os.Open(genesisFile)
			if err != nil {
				return err
			}
			defer genesisJSON.Close()

			byteValue, err := ioutil.ReadAll(genesisJSON)
			if err != nil {
				return err
			}

			claimsGen, err := getClaimGenesisState(byteValue)
			if err != nil {
				return err
			}

			fmt.Printf("Total claims %d \n", len(claimsGen.ClaimsRecords))

			return nil
		},
	}

	return cmd
}
