# Enhanced Placeholder Syntax Design

**Date:** 2025-10-30
**Status:** Approved
**Author:** Design Session with User

## Overview

This design extends the current placeholder syntax (`{{key}}` and `{{key|default}}`) with two new features:
1. **No-save flag**: Prevent specific placeholder values from being saved to history
2. **Conditional wrapper text**: Prefix/suffix text that only appears when placeholder value is non-empty

## Motivation

**Current Limitations:**
- All placeholder values are automatically saved to history (storage.ts:393-434)
- No way to prevent history pollution from one-off values (dates, temporary IDs, timestamps)
- No conditional text rendering based on placeholder value presence
- Users need wrapper text for natural language flow (e.g., "value" vs "value with context")

**User Requirements:**
- Some placeholders are ephemeral and shouldn't clutter history
- Wrapper text should conditionally appear only when placeholder has a value
- Separate concerns: history control is independent from wrapper text
- Backwards compatibility not required (syntax can be refactored)

## Design Decision: Order-Based Syntax

### Syntax Specification

**Complete Pattern:**
```
{{[!]prefix:key:suffix[|default]}}
```

**Components:**
- `!` (optional): No-save flag - value won't be saved to placeholder history
- `prefix` (optional): Text shown before value (only when value is non-empty)
- `key` (required): Placeholder identifier
- `suffix` (optional): Text shown after value (only when value is non-empty)
- `|default` (optional): Default value for optional placeholders

**Parsing Rules:**
- If colons are present, must have exactly 2 colons creating 3 parts
- **Middle part is always the key**
- First part (before first `:`) is prefix wrapper
- Third part (after second `:`) is suffix wrapper
- Empty strings are valid: `{{:key:}}` = no wrappers
- `|` splits on rightmost occurrence for default value

### Syntax Examples

**Basic Placeholders:**
```
{{name}}                    → Required, saved to history
{{!name}}                   → Required, NOT saved to history
{{name|John}}               → Optional with default, saved
{{!name|John}}              → Optional with default, NOT saved
{{:name:}}                  → Explicit no-wrappers (equivalent to {{name}})
```

**Wrapper Text:**
```
{{#:id:}}                   → Prefix "#", no suffix
{{:id:#}}                   → No prefix, suffix "#"
{{#:id:#}}                  → Both: prefix "#", suffix "#"
{{with :context:}}          → Prefix "with " (note trailing space)
{{:context: here}}          → Suffix " here" (note leading space)
```

**Combined Features:**
```
{{!:timestamp::ms}}         → Suffix ":ms", not saved
{{$:amount: USD}}           → Prefix "$", suffix " USD"
{{!#:temp_id:|unknown}}     → Prefix "#", not saved, default "unknown"
```

**Real-World Examples:**
```
Hello {{:name:}}, your order {{#:order_id:}} is ready!
→ Input: name="Alice", order_id="12345"
→ Output: "Hello Alice, your order #12345 is ready!"

Message{{with :context:}}
→ Input: context="urgent"
→ Output: "Message with urgent"
→ Input: context=""
→ Output: "Message"

Price: {{$:price: USD|0.00}}
→ Input: price="25.50"
→ Output: "Price: $25.50 USD"
→ Input: price=""
→ Output: "Price: 0.00"

Event on {{!:date:}} at {{!:time::pm}}
→ Input: date="2025-10-30", time="3"
→ Output: "Event on 2025-10-30 at 3:pm"
→ Note: date and time NOT saved to history
```

## Architecture Changes

### Type System Updates

**File:** `src/types.ts`

**Updated `Placeholder` interface:**
```typescript
export interface Placeholder {
  key: string;
  defaultValue?: string;
  isRequired: boolean;
  isSaved: boolean;          // NEW: Whether to save to history
  prefixWrapper?: string;    // NEW: Prefix text (conditional on non-empty value)
  suffixWrapper?: string;    // NEW: Suffix text (conditional on non-empty value)
}
```

### Parsing Logic

**File:** `src/utils/placeholders.ts`

**Function:** `extractPlaceholders(text: string): Placeholder[]`

**Algorithm:**
1. Match all `{{...}}` patterns with regex
2. For each match:
   a. Check for `!` prefix → set `isSaved = false`
   b. Split on rightmost `|` to extract default value
   c. Parse core content:
      - If no colons: entire content is key
      - If exactly 2 colons: split into [prefix, key, suffix]
      - If 1 or 3+ colons: treat as key (invalid syntax, backwards compat)
   d. Build `Placeholder` object

**Edge Cases:**
- Empty prefix/suffix (`{{:key:}}` or `{{prefix:key:}}`) → Convert empty strings to `undefined`
- Invalid colon count → Treat entire content as key
- Multiple `|` characters → Split on rightmost only
- Whitespace handling → Trim key, preserve wrapper whitespace

### Replacement Logic

**File:** `src/utils/placeholders.ts`

**Function:** `replacePlaceholders(text: string, values: Record<string, string>, placeholders: Placeholder[]): string`

**Algorithm:**
1. For each placeholder:
   a. Determine final value: `values[key] ?? defaultValue ?? ""`
   b. Build replacement:
      - If `value.trim()` is empty: use empty string (no wrappers)
      - If `value.trim()` is non-empty: apply `prefixWrapper + value + suffixWrapper`
   c. Replace all occurrences in text using regex

**Key Behavior:**
- **Wrappers only apply to non-empty values** (including whitespace-only values treated as empty)
- Empty/undefined values → no wrappers rendered
- Default values → wrappers apply if default is non-empty

### Storage Integration

**File:** `src/components/PlaceholderForm.tsx`

**Function:** `handleSubmit(values: Record<string, string>)`

**Changes:**
```typescript
// Current code (lines 109-115):
for (const placeholder of props.placeholders) {
  const value = finalValues[placeholder.key];
  if (value && value.trim()) {
    await addPlaceholderValue(placeholder.key, value);
  }
}

// Updated code:
for (const placeholder of props.placeholders) {
  const value = finalValues[placeholder.key];
  // Only save if isSaved flag is true
  if (placeholder.isSaved && value && value.trim()) {
    await addPlaceholderValue(placeholder.key, value);
  }
}
```

## User Interface Updates

### 1. Inline Syntax Reference

**File:** `src/components/SnippetForm.tsx` (or wherever snippet create/edit form exists)

**Location:** Below the Content field

**Implementation:**
```tsx
<Form.TextField
  id="content"
  title="Content"
  placeholder="Enter snippet content..."
  value={content}
  onChange={setContent}
/>
<Form.Description
  text="Placeholders: {{key}} (required) | {{key|default}} (optional) | {{prefix:key:suffix}} (wrappers) | {{!key}} (no history save)"
/>
```

**Purpose:** Quick reference for users while writing snippets without leaving the form.

### 2. Detailed Help Documentation

**File:** `src/components/SearchOperatorsHelp.tsx`

**New Section:** Add "Placeholder Syntax" section (similar to existing "Tag Operators", "Boolean Operators", etc.)

**Section Items:**
1. **Overview** - Introduction to placeholder system
2. **{{key}}** - Required placeholder
3. **{{key|default}}** - Optional with default
4. **{{prefix:key:suffix}}** - Conditional wrappers
5. **{{!key}}** - No-save flag
6. **Complex Examples** - Combining all features

**Each item includes:**
- Title and subtitle
- Detailed markdown documentation
- Examples with input/output
- Use cases
- Related syntax patterns

**Example structure:**
```tsx
<List.Section title="Placeholder Syntax">
  <List.Item
    icon={{ source: Icon.CodeBlock, tintColor: Color.Blue }}
    title="{{key}}"
    subtitle="Required placeholder"
    detail={
      <List.Item.Detail
        markdown={`
# {{key}}

Basic required placeholder...

## Examples
...
        `}
      />
    }
  />
  {/* More items... */}
</List.Section>
```

### 3. Form Field Hints

**File:** `src/components/PlaceholderForm.tsx`

**Location:** Form field `info` props

**Update to show placeholder metadata:**
- Show if placeholder is saved to history
- Show wrapper text if present
- Example: `"Optional (default: \"none\") | Prefix: \"$\" | Not saved to history"`

## Validation and Error Handling

### Parsing Validation

**Invalid Syntax Examples:**
- `{{key:value}}` → 1 colon: treat as key "key:value"
- `{{a:b:c:d}}` → 3+ colons: treat as key "a:b:c:d"
- `{{!}}` → Empty key: skip this placeholder
- `{{:}}` → Only colon: skip this placeholder

**Graceful Degradation:**
- Invalid syntax treated as literal key (backwards compatible)
- Empty keys are skipped
- Parsing never throws errors

### Form Validation

**No changes required:**
- Required field validation (lines 93-103) remains unchanged
- `isRequired` flag still based on presence of `|` delimiter

## Migration Strategy

### No Migration Needed

**User confirmed:** Backwards compatibility not required. Existing snippets can be refactored.

**Approach:**
- Update parser to handle both old and new syntax
- Old syntax (`{{key}}`, `{{key|default}}`) continues to work
- No data migration required
- Users update snippets organically as needed

### Default Values for New Fields

**New placeholders:**
- `isSaved: true` (default behavior)
- `prefixWrapper: undefined` (no wrapper)
- `suffixWrapper: undefined` (no wrapper)

## Testing Strategy

### Unit Tests for Parsing

**File:** `src/utils/placeholders.test.ts`

**Test Cases:**
1. Basic syntax: `{{key}}`, `{{key|default}}`
2. No-save flag: `{{!key}}`, `{{!key|default}}`
3. Wrappers: `{{prefix:key:}}`, `{{:key:suffix}}`, `{{prefix:key:suffix}}`
4. Combined: `{{!prefix:key:suffix|default}}`
5. Edge cases: empty strings, whitespace, invalid colon counts
6. Backwards compat: existing syntax still works

### Unit Tests for Replacement

**Test Cases:**
1. Empty values don't render wrappers
2. Non-empty values render wrappers correctly
3. Default values work with wrappers
4. No-save flag doesn't affect replacement output
5. Whitespace in wrappers preserved

### Integration Tests

**Scenarios:**
1. Create snippet with new syntax → Fill form → Copy → Verify output
2. Create snippet with `!` flag → Fill form → Verify NOT saved to history
3. Create snippet with wrappers → Test empty vs filled values
4. Mix old and new syntax in single snippet

## Performance Considerations

**Minimal Impact:**
- Parsing: 3-5 additional operations per placeholder (colon splitting, flag checking)
- Replacement: 2 additional string concatenations per non-empty placeholder with wrappers
- Storage: Skip saving for `!` flagged placeholders (reduces writes)

**Expected Performance:**
- Negligible overhead for typical snippets (1-5 placeholders)
- Storage writes reduced for ephemeral placeholders

## Documentation Updates

### README.md

**Section: "Using Placeholders"**

**Update to:**
```markdown
## Using Placeholders

Create dynamic snippet templates with enhanced placeholder syntax:

### Basic Syntax
- **Required**: `{{name}}` - prompts for value when copying
- **Optional**: `{{name|John Doe}}` - with default value
- **No-save**: `{{!date}}` - value NOT saved to history

### Conditional Wrapper Text
- **Prefix**: `{{#:id:}}` - adds "#" before value (only if value present)
- **Suffix**: `{{:id:#}}` - adds "#" after value (only if value present)
- **Both**: `{{$:price: USD}}` - adds "$" before and " USD" after

### Examples
```
Hello {{:name:}}, your order {{#:order_id:}} is ready!
→ "Hello Alice, your order #12345 is ready!"

Message{{with :context:|}}
→ If context filled: "Message with urgent details"
→ If context empty: "Message"

Event on {{!:date:}} at {{!:time::pm}}
→ date/time not saved to history
```

When you copy a snippet with placeholders, you'll be prompted to fill in the values.
Values are saved to history for autocomplete (unless `!` flag is used).
```

## Implementation Phases

### Phase 1: Core Parsing
1. Update `Placeholder` interface in types.ts
2. Implement new parsing logic in placeholders.ts
3. Write unit tests for parsing
4. Verify backwards compatibility

### Phase 2: Replacement Logic
1. Update `replacePlaceholders()` with wrapper rendering
2. Write unit tests for replacement
3. Test edge cases (empty values, whitespace)

### Phase 3: Storage Integration
1. Update `PlaceholderForm.tsx` to respect `isSaved` flag
2. Test history saving behavior

### Phase 4: UI Updates
1. Add inline reference to snippet form
2. Add placeholder syntax section to SearchOperatorsHelp.tsx
3. Update form field hints with placeholder metadata

### Phase 5: Documentation
1. Update README.md
2. Add examples to help documentation
3. Test documentation clarity with real usage

## Open Questions

None - design approved and ready for implementation.

## Alternative Designs Considered

### Approach 2: Verbose Keyword-Based Syntax
```
{{key~nosave~prefix="text"~default="value"}}
```
**Rejected:** Too verbose, tedious to type, harder to visually scan.

### Approach 3: Bracket-Based Wrapper Syntax
```
{{![prefix]key|default}}
```
**Rejected:** Ambiguous whether `{{key[suffix]}}` is prefix or suffix. Brackets add visual weight.

### Chosen: Order-Based with Colons
```
{{!prefix:key:suffix|default}}
```
**Selected:** Consistent structure, unambiguous parsing, reasonable verbosity, clear precedence.

## References

- Current implementation: src/utils/placeholders.ts (lines 1-62)
- Storage integration: src/components/PlaceholderForm.tsx (lines 108-115)
- Type definitions: src/types.ts (lines 35-39)
- Recent enhancement: commit f4712e9 (empty string defaults with `{{key|}}` syntax)
