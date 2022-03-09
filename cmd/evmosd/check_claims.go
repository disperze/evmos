package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/spf13/cobra"

	"github.com/cosmos/cosmos-sdk/codec"
	claimtypes "github.com/tharsis/evmos/v2/x/claims/types"
)

func getClaimGenesisState(genesis []byte, cdc codec.JSONCodec) (claimtypes.GenesisState, error) {
	var gen map[string]json.RawMessage
	var claimGen claimtypes.GenesisState

	err := json.Unmarshal(genesis, &gen)
	if err != nil {
		return claimGen, err
	}

	var appState map[string]json.RawMessage
	err = json.Unmarshal(gen["app_state"], &appState)
	if err != nil {
		return claimGen, err
	}

	err = cdc.UnmarshalJSON(appState[claimtypes.ModuleName], &claimGen)
	if err != nil {
		return claimGen, err
	}

	return claimGen, nil
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

			genesisJSON, err := os.Open(genesisFile)
			if err != nil {
				return err
			}
			defer genesisJSON.Close()

			byteValue, err := ioutil.ReadAll(genesisJSON)
			if err != nil {
				return err
			}

			claimsGen, err := getClaimGenesisState(byteValue, cdc)
			if err != nil {
				return err
			}

			fmt.Printf("Total claims %d \n", len(claimsGen.ClaimsRecords))

			return nil
		},
	}

	return cmd
}
