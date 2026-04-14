package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/hr/internal/config"
	"github.com/sglavoie/dev-helpers/go/hr/internal/storage"
	"github.com/sglavoie/dev-helpers/go/hr/internal/tui"
	"github.com/spf13/cobra"
)

var (
	addExercise string
	addReps     int
	addRounds   int
	addNotes    string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Log an exercise",
	Long:  "Interactively select an exercise and log reps/rounds. Use flags for non-interactive mode.",
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringVarP(&addExercise, "exercise", "e", "", "exercise name (skip TUI selector)")
	addCmd.Flags().IntVarP(&addReps, "reps", "r", 0, "number of reps (0 = use config default)")
	addCmd.Flags().IntVarP(&addRounds, "rounds", "R", 0, "number of rounds (0 = use config default)")
	addCmd.Flags().StringVarP(&addNotes, "notes", "n", "", "optional notes")
}

func runAdd(cmd *cobra.Command, args []string) error {
	cfgPath, err := config.ConfigPath()
	if err != nil {
		return err
	}
	cfg, err := config.LoadOrInit(cfgPath)
	if err != nil {
		return fmt.Errorf("loading config: %w", err)
	}

	var exercise config.Exercise

	if addExercise == "" {
		exercise, err = tui.RunSelector(cfg.Exercises)
		if err != nil {
			return err
		}
	} else {
		exercise, err = findExercise(cfg.Exercises, addExercise)
		if err != nil {
			return err
		}
	}

	reps := addReps
	if reps == 0 {
		reps, err = promptInt("Reps", exercise.DefaultReps)
		if err != nil {
			return fmt.Errorf("reading reps: %w", err)
		}
	}

	rounds := addRounds
	if rounds == 0 {
		rounds, err = promptInt("Rounds", cfg.DefaultRounds)
		if err != nil {
			return fmt.Errorf("reading rounds: %w", err)
		}
	}

	entry := storage.Entry{
		Timestamp: time.Now().UTC(),
		Exercise:  exercise.Name,
		Reps:      reps,
		Rounds:    rounds,
		Notes:     addNotes,
	}

	dataPath, err := config.DataPath()
	if err != nil {
		return err
	}
	if err := storage.Append(dataPath, entry); err != nil {
		return fmt.Errorf("saving entry: %w", err)
	}

	fmt.Printf("Logged: %s x%d x%d rounds\n", exercise.Name, reps, rounds)
	return nil
}

func findExercise(exercises []config.Exercise, name string) (config.Exercise, error) {
	for _, ex := range exercises {
		if strings.EqualFold(ex.Name, name) {
			return ex, nil
		}
	}
	return config.Exercise{}, fmt.Errorf("exercise %q not found in config; run without --exercise to use the selector", name)
}

func promptInt(label string, defaultVal int) (int, error) {
	fmt.Printf("%s [%d]: ", label, defaultVal)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	n, err := strconv.Atoi(line)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", line)
	}
	return n, nil
}
