# Integration Tests

This directory contains integration tests that verify complete workflows and component interactions in the qf application.

## Configuration Management Test (`config_test.go`)

### Overview
Tests the complete configuration management workflow covering:
1. **Configuration Editing**: Opening and modifying configuration files
2. **Settings Validation**: Ensuring invalid configurations are rejected
3. **Hot-reload**: Applying configuration changes without application restart
4. **Behavior Verification**: Confirming settings affect application behavior
5. **Multi-component Updates**: Ensuring all components receive configuration updates

### Test Scenarios

#### Workflow 4: Configuration Management
Based on the quickstart scenario:
1. **Open Config**: `qf --config edit` - Configuration editor opens
2. **Modify Settings**: Change debounce delay to 100ms, increase cache size
3. **Apply Changes**: Save configuration file - Hot-reload applies changes
4. **Test Changes**: Verify faster response time and new debounce delay

### Test Structure

```go
TestConfigurationManagement
├── OpenConfiguration       # Tests config file access and editing
├── ModifySettings          # Tests changing debounce delay and cache size
├── ApplyChanges           # Tests hot-reload without restart
├── VerifyBehaviorChanges  # Tests settings affect application behavior
├── ConfigurationValidation # Tests invalid configurations are rejected
└── HotReloadIntegration   # Tests multiple components receive updates
```

### Mock Components

The test uses comprehensive mocks to simulate the real application:

- **MockConfig**: Complete configuration structure matching the real config schema
- **MockConfigWatcher**: Simulates file system watching for configuration changes
- **MockApplication**: Simulates main application using configuration settings
- **MockConfigValidator**: Simulates configuration validation logic
- **MockFilterEngine**: Simulates filter engine component receiving updates
- **MockUIManager**: Simulates UI manager component receiving updates
- **MockSessionManager**: Simulates session manager component receiving updates
- **MockConfigManager**: Coordinates configuration updates across components

### Configuration Schema Tested

```json
{
  "version": "1.0.0",
  "performance": {
    "debounce_delay_ms": 200,
    "cache_size_mb": 10,
    "streaming_threshold_mb": 100,
    "max_workers": 4
  },
  "ui": {
    "theme": "default",
    "show_line_numbers": true,
    "highlight_colors": ["red", "green", "blue"]
  },
  "data_management": {
    "session_retention_days": 30,
    "max_history_entries": 100
  },
  "file_handling": {
    "default_encoding": "utf-8",
    "max_file_size": "1GB"
  }
}
```

### Validation Rules Tested

- **debounce_delay_ms**: Must be between 50 and 1000 milliseconds
- **cache_size_mb**: Must be positive
- **version**: Cannot be empty
- All configuration changes must pass validation before applying

### Success Criteria

The test verifies:
- ✅ Configuration files can be opened and edited
- ✅ Settings modifications are persisted correctly
- ✅ Hot-reload detects changes without application restart
- ✅ Application behavior reflects new configuration values
- ✅ Invalid configurations are rejected with meaningful errors
- ✅ Multiple application components receive configuration updates
- ✅ Debounce delay affects filter update timing
- ✅ Cache size affects memory allocation

### Running the Test

```bash
# Run just the configuration test
go test -v ./tests/integration/config_test.go

# Run all integration tests
go test -v ./tests/integration/

# Run with specific test case
go test -v ./tests/integration/ -run TestConfigurationManagement/OpenConfiguration
```

### Current Status

The test is currently **SKIPPED** because the configuration management implementation does not exist yet. Remove the `t.Skip()` line in `TestConfigurationManagement()` once the implementation is ready.

### Implementation Requirements

When implementing the actual configuration management system, ensure:

1. **Config Structure**: Match the `MockConfig` structure and JSON schema
2. **File Watching**: Implement file system watching for hot-reload
3. **Validation**: Implement the validation rules tested here
4. **Component Updates**: Ensure all components can receive configuration updates
5. **Performance**: Respect debounce delays and cache size settings
6. **Error Handling**: Provide meaningful error messages for invalid configurations

### Future Enhancements

Potential additions to this test:
- Configuration backup and rollback scenarios
- Multiple configuration file support
- Environment variable override testing
- Configuration migration testing
- Performance benchmarking for hot-reload