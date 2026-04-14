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
	addFields   []string
	addNotes    string
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "Log an exercise",
	Long:  "Interactively select an exercise and log fields. Use flags for non-interactive mode.",
	RunE:  runAdd,
}

func init() {
	addCmd.Flags().StringVarP(&addExercise, "exercise", "e", "", "exercise name (skip TUI selector)")
	addCmd.Flags().StringArrayVarP(&addFields, "field", "f", nil, "field value as key=value (repeatable)")
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
		exercise, err = tui.RunSelector(cfg)
		if err != nil {
			return err
		}
	} else {
		exercise, err = findExercise(cfg.Exercises, addExercise)
		if err != nil {
			return err
		}
	}

	// Parse --field flags into a map
	flagFields := make(map[string]string)
	for _, f := range addFields {
		k, v, ok := strings.Cut(f, "=")
		if !ok {
			return fmt.Errorf("invalid field format %q, expected key=value", f)
		}
		flagFields[k] = v
	}

	data := make(map[string]any)
	for _, field := range exercise.Fields {
		if strVal, ok := flagFields[field.Name]; ok {
			val, err := parseFieldValue(field, strVal)
			if err != nil {
				return err
			}
			data[field.Name] = val
		} else {
			val, err := promptField(field)
			if err != nil {
				return fmt.Errorf("reading %s: %w", field.Name, err)
			}
			data[field.Name] = val
		}
	}

	entry := storage.Entry{
		Timestamp: time.Now().UTC(),
		Exercise:  exercise.Name,
		Data:      data,
		Notes:     addNotes,
	}

	dataPath, err := config.DataPath()
	if err != nil {
		return err
	}
	if err := storage.Append(dataPath, entry); err != nil {
		return fmt.Errorf("saving entry: %w", err)
	}

	var parts []string
	for _, field := range exercise.Fields {
		parts = append(parts, fmt.Sprintf("%s: %v", field.Name, data[field.Name]))
	}
	fmt.Printf("Logged: %s (%s)\n", exercise.Name, strings.Join(parts, ", "))
	return nil
}

func parseFieldValue(field config.Field, s string) (any, error) {
	switch field.Type {
	case config.FieldTypeInt:
		n, err := strconv.Atoi(s)
		if err != nil {
			return nil, fmt.Errorf("field %q: invalid int %q", field.Name, s)
		}
		return n, nil
	case config.FieldTypeFloat:
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, fmt.Errorf("field %q: invalid float %q", field.Name, s)
		}
		return f, nil
	default:
		return s, nil
	}
}

func promptField(field config.Field) (any, error) {
	switch field.Type {
	case config.FieldTypeInt:
		defaultVal := 0
		if v, ok := field.Default.(int); ok {
			defaultVal = v
		} else if v, ok := field.Default.(float64); ok {
			defaultVal = int(v)
		}
		return promptInt(field.Name, defaultVal)
	case config.FieldTypeFloat:
		defaultVal := 0.0
		if v, ok := field.Default.(float64); ok {
			defaultVal = v
		} else if v, ok := field.Default.(int); ok {
			defaultVal = float64(v)
		}
		return promptFloat(field.Name, defaultVal)
	default:
		defaultVal := ""
		if v, ok := field.Default.(string); ok {
			defaultVal = v
		}
		return promptString(field.Name, defaultVal)
	}
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

func promptFloat(label string, defaultVal float64) (float64, error) {
	fmt.Printf("%s [%g]: ", label, defaultVal)
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	f, err := strconv.ParseFloat(line, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid number %q", line)
	}
	return f, nil
}

func promptString(label string, defaultVal string) (string, error) {
	if defaultVal != "" {
		fmt.Printf("%s [%s]: ", label, defaultVal)
	} else {
		fmt.Printf("%s: ", label)
	}
	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return defaultVal, nil
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return defaultVal, nil
	}
	return line, nil
}
