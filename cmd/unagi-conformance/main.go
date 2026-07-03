// Command unagi-conformance drives the differential fixture corpus: run
// fixtures in golden or live mode, record oracle goldens from the pinned
// CPython, and report per-band coverage. The harness itself lives in
// pkg/conformance; this layer parses arguments and prints reports.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/tamnd/unagi/pkg/conformance"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	root := &cobra.Command{
		Use:           "unagi-conformance",
		Short:         "Differential conformance harness for unagi",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().String("root", "conformance/fixtures", "fixture corpus directory")
	root.PersistentFlags().String("ledger", "compat/ledger.yaml", "divergence ledger file")
	root.PersistentFlags().String("pin", "conformance/ORACLE_PIN", "oracle pin file")
	root.PersistentFlags().String("python", "python3", "oracle interpreter for live mode and record")
	root.AddCommand(newFixturesCmd(), newRecordCmd())
	if err := root.ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "unagi-conformance:", err)
		os.Exit(2)
	}
}

// selectFixtures applies --only and --band to the discovered corpus.
func selectFixtures(cmd *cobra.Command) ([]conformance.Fixture, error) {
	rootDir, _ := cmd.Flags().GetString("root")
	fixtures, err := conformance.Discover(rootDir)
	if err != nil {
		return nil, err
	}
	if only, _ := cmd.Flags().GetString("only"); only != "" {
		id, err := strconv.Atoi(only)
		if err != nil {
			return nil, fmt.Errorf("--only takes a fixture id, got %q", only)
		}
		for _, f := range fixtures {
			if f.ID == id {
				return []conformance.Fixture{f}, nil
			}
		}
		return nil, fmt.Errorf("no fixture with id %04d", id)
	}
	if band, _ := cmd.Flags().GetString("band"); band != "" {
		lo, hi, ok := strings.Cut(band, "-")
		if !ok {
			return nil, fmt.Errorf("--band takes lo-hi, got %q", band)
		}
		l, err1 := strconv.Atoi(lo)
		h, err2 := strconv.Atoi(hi)
		if err1 != nil || err2 != nil {
			return nil, fmt.Errorf("--band takes lo-hi, got %q", band)
		}
		var out []conformance.Fixture
		for _, f := range fixtures {
			if f.ID >= l && f.ID <= h {
				out = append(out, f)
			}
		}
		return out, nil
	}
	return fixtures, nil
}

func newRunner(cmd *cobra.Command) (*conformance.Runner, error) {
	ledgerPath, _ := cmd.Flags().GetString("ledger")
	python, _ := cmd.Flags().GetString("python")
	keepTmp, _ := cmd.Flags().GetBool("keep-tmp")
	ids, err := conformance.LoadLedgerIDs(ledgerPath)
	if err != nil {
		return nil, err
	}
	return &conformance.Runner{Python: python, KeepTmp: keepTmp, LedgerIDs: ids}, nil
}

func newFixturesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "fixtures",
		Short: "Run the fixture corpus and judge each against the oracle",
		RunE: func(cmd *cobra.Command, args []string) error {
			fixtures, err := selectFixtures(cmd)
			if err != nil {
				return err
			}
			runner, err := newRunner(cmd)
			if err != nil {
				return err
			}
			live, _ := cmd.Flags().GetBool("live")
			coverage, _ := cmd.Flags().GetBool("coverage")
			if live {
				if err := checkPin(cmd); err != nil {
					return err
				}
			}
			results := map[int]conformance.Result{}
			failed := 0
			for _, f := range fixtures {
				var res conformance.Result
				if live {
					res = runner.RunLive(cmd.Context(), f)
				} else {
					res = runner.RunGolden(cmd.Context(), f)
				}
				results[f.ID] = res
				conformance.PrintResult(cmd.OutOrStdout(), f, res, live)
				if !res.Skipped && res.Verdict != conformance.Pass && res.Verdict != conformance.DivergentOK {
					failed++
				}
			}
			if coverage {
				conformance.PrintCoverage(cmd.OutOrStdout(), results)
			}
			cmd.Printf("%d fixtures, %d failing\n", len(fixtures), failed)
			if failed > 0 {
				os.Exit(1)
			}
			return nil
		},
	}
	cmd.Flags().Bool("live", false, "run the pinned CPython per fixture instead of reading goldens")
	cmd.Flags().Bool("coverage", false, "print per-band pass rates")
	cmd.Flags().Bool("keep-tmp", false, "keep run directories for debugging")
	cmd.Flags().String("only", "", "run a single fixture by id")
	cmd.Flags().String("band", "", "run one band as lo-hi, e.g. 1400-1599")
	return cmd
}

func newRecordCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "record",
		Short: "Regenerate oracle.golden files from the pinned CPython",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkPin(cmd); err != nil {
				return err
			}
			fixtures, err := selectFixtures(cmd)
			if err != nil {
				return err
			}
			runner, err := newRunner(cmd)
			if err != nil {
				return err
			}
			changed := 0
			for _, f := range fixtures {
				ch, err := runner.Record(cmd.Context(), f)
				if err != nil {
					return fmt.Errorf("record %s: %v", f.Name, err)
				}
				if ch {
					changed++
					cmd.Printf("recorded %s\n", f.Name)
				}
			}
			cmd.Printf("recorded %d fixtures, %d changed\n", len(fixtures), changed)
			return nil
		},
	}
	cmd.Flags().String("only", "", "record a single fixture by id")
	cmd.Flags().String("band", "", "record one band as lo-hi")
	cmd.Flags().Bool("keep-tmp", false, "keep run directories for debugging")
	return cmd
}

func checkPin(cmd *cobra.Command) error {
	pinPath, _ := cmd.Flags().GetString("pin")
	python, _ := cmd.Flags().GetString("python")
	pinned, err := conformance.ReadPin(pinPath)
	if err != nil {
		return err
	}
	full, err := conformance.CheckPin(cmd.Context(), python, pinned)
	if err != nil {
		return err
	}
	cmd.Printf("oracle: Python %s [pinned: OK]\n", strings.Split(full, "\n")[0])
	return nil
}
