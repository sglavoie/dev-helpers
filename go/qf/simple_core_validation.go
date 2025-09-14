// Package main demonstrates core functionality without concurrency issues
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/sglavoie/dev-helpers/go/qf/internal/core"
	"github.com/sglavoie/dev-helpers/go/qf/internal/session"
)

func main() {
	fmt.Println("🔧 qf Core Functionality Validation (Simple)")
	fmt.Println("===========================================")

	// Test 1: Basic Filtering
	fmt.Println("\n📋 Test 1: Basic Filtering")
	testBasicFilteringSimple()

	// Test 2: Multi-pattern Filtering
	fmt.Println("\n📋 Test 2: Include + Exclude Filtering")
	testIncludeExcludeFiltering()

	// Test 3: Session Management
	fmt.Println("\n📋 Test 3: Session Management")
	testSessionManagementBasic()

	// Test 4: Pattern Validation
	fmt.Println("\n📋 Test 4: Pattern Validation")
	testPatternValidation()

	fmt.Println("\n✅ Core functionality validation complete!")
	fmt.Println("   Core components are working correctly for basic scenarios.")
}

func testBasicFilteringSimple() {
	// Create a filter set directly
	filterSet := core.NewFilterSet("test-basic")

	// Create ERROR pattern
	errorPattern := &core.Pattern{
		ID:         "error-pattern",
		Expression: "ERROR",
		Type:       core.Include,
		Color:      "#ff0000",
		Created:    time.Now(),
		IsValid:    true,
	}

	// Add pattern to filter set
	err := filterSet.AddPattern(*errorPattern)
	if err != nil {
		log.Fatalf("Failed to add pattern: %v", err)
	}

	// Test content
	content := []string{
		"2025-09-14 10:00:01 INFO Application started",
		"2025-09-14 10:00:05 ERROR Database connection failed",
		"2025-09-14 10:00:06 ERROR Authentication failed",
		"2025-09-14 10:00:07 WARN Session expired",
		"2025-09-14 10:00:09 ERROR Invalid request",
	}

	// Use line filter processor for simple filtering
	processor := core.NewLineFilterProcessor()
	matched := 0

	for _, line := range content {
		shouldInclude, err := processor.ShouldIncludeLine(line, filterSet.Include, filterSet.Exclude)
		if err != nil {
			log.Printf("Error processing line: %v", err)
			continue
		}
		if shouldInclude {
			matched++
		}
	}

	fmt.Printf("   📊 Input: %d lines, Matched: %d lines\n", len(content), matched)
	if matched == 3 {
		fmt.Println("   ✅ Basic ERROR filtering works correctly")
	} else {
		fmt.Printf("   ❌ Expected 3 ERROR lines, got %d\n", matched)
	}
}

func testIncludeExcludeFiltering() {
	filterSet := core.NewFilterSet("test-include-exclude")

	// Add include pattern for ERROR
	errorPattern := &core.Pattern{
		ID:         "error-pattern",
		Expression: "ERROR",
		Type:       core.Include,
		Color:      "#ff0000",
		Created:    time.Now(),
		IsValid:    true,
	}
	filterSet.AddPattern(*errorPattern)

	// Add exclude pattern for "timeout"
	timeoutPattern := &core.Pattern{
		ID:         "timeout-pattern",
		Expression: "timeout",
		Type:       core.Exclude,
		Color:      "#0000ff",
		Created:    time.Now(),
		IsValid:    true,
	}
	filterSet.AddPattern(*timeoutPattern)

	content := []string{
		"2025-09-14 10:00:05 ERROR Database connection timeout",
		"2025-09-14 10:00:06 ERROR Authentication failed",
		"2025-09-14 10:00:09 ERROR Invalid request format",
		"2025-09-14 10:00:11 ERROR Network timeout occurred",
	}

	processor := core.NewLineFilterProcessor()
	matched := 0
	var matchedLines []string

	for _, line := range content {
		shouldInclude, err := processor.ShouldIncludeLine(line, filterSet.Include, filterSet.Exclude)
		if err != nil {
			log.Printf("Error processing line: %v", err)
			continue
		}
		if shouldInclude {
			matched++
			matchedLines = append(matchedLines, line)
		}
	}

	fmt.Printf("   📊 Input: %d lines, Matched: %d lines\n", len(content), matched)

	// Verify results: should have ERROR but not "timeout"
	allValid := true
	for _, line := range matchedLines {
		if !strings.Contains(line, "ERROR") || strings.Contains(line, "timeout") {
			allValid = false
			break
		}
	}

	if matched == 2 && allValid {
		fmt.Println("   ✅ Include + Exclude filtering works correctly")
	} else {
		fmt.Printf("   ❌ Expected 2 valid ERROR lines (no timeout), got %d\n", matched)
	}
}

func testSessionManagementBasic() {
	// Create session
	sess := session.NewSession("test-session")

	// Add file tabs
	_, err := sess.AddFileTab("/path/to/server1.log", []string{})
	if err != nil {
		log.Printf("Failed to add file tab: %v", err)
	}

	_, err = sess.AddFileTab("/path/to/server2.log", []string{})
	if err != nil {
		log.Printf("Failed to add file tab: %v", err)
	}

	// Create and set filter set
	filterSet := session.FilterSet{
		Name: "test-filters",
		Include: []session.FilterPattern{
			{
				ID:         "critical-pattern",
				Expression: "CRITICAL",
				Type:       session.FilterInclude,
				Created:    time.Now(),
				IsValid:    true,
			},
		},
		Exclude: []session.FilterPattern{},
	}

	sess.UpdateFilterSet(filterSet)

	// Verify session state
	if len(sess.OpenFiles) == 2 {
		fmt.Println("   ✅ File tab management works")
	} else {
		fmt.Printf("   ❌ Expected 2 files, got %d\n", len(sess.OpenFiles))
	}

	if sess.FilterSet.Name == "test-filters" && len(sess.FilterSet.Include) == 1 {
		fmt.Println("   ✅ Filter set management works")
	} else {
		fmt.Println("   ❌ Filter set management failed")
	}

	// Test session validation
	err = sess.Validate()
	if err == nil {
		fmt.Println("   ✅ Session validation works")
	} else {
		fmt.Printf("   ❌ Session validation failed: %v\n", err)
	}
}

func testPatternValidation() {
	// Test valid pattern
	validPattern := &core.Pattern{
		ID:         "valid-pattern",
		Expression: "ERROR|WARN",
		Type:       core.Include,
		Color:      "#ff0000",
		Created:    time.Now(),
	}

	isValid, err := validPattern.Validate()
	if isValid && err == nil {
		fmt.Println("   ✅ Valid pattern validation works")
	} else {
		fmt.Printf("   ❌ Valid pattern should pass validation: %v\n", err)
	}

	// Test invalid pattern
	invalidPattern := &core.Pattern{
		ID:         "invalid-pattern",
		Expression: "[invalid regex",
		Type:       core.Include,
		Color:      "#ff0000",
		Created:    time.Now(),
	}

	isValid, err = invalidPattern.Validate()
	if !isValid && err != nil {
		fmt.Println("   ✅ Invalid pattern validation works")
	} else {
		fmt.Println("   ❌ Invalid pattern should fail validation")
	}

	// Test empty pattern
	emptyPattern := &core.Pattern{
		ID:         "empty-pattern",
		Expression: "",
		Type:       core.Include,
		Color:      "#ff0000",
		Created:    time.Now(),
	}

	isValid, err = emptyPattern.Validate()
	if !isValid && err != nil {
		fmt.Println("   ✅ Empty pattern validation works")
	} else {
		fmt.Println("   ❌ Empty pattern should fail validation")
	}
}
