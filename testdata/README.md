# Test Workflows

Sample workflows for manual testing of gh-wfd features.

## Workflow Coverage

### Basic Scenarios

| Workflow | Purpose | Key Features |
|----------|---------|--------------|
| `simple-dispatch.yml` | Basic dispatchable workflow | No inputs, minimal configuration |
| `minimal.yml` | Minimal valid workflow | Missing name field, uses filename as fallback |
| `ci.yml` | Non-dispatchable workflow | Should be filtered out (push/PR triggers only) |
| `not-dispatchable.yml` | Another non-dispatchable | Should be filtered out (schedule trigger) |

### Input Type Testing

| Workflow | Purpose | Input Types |
|----------|---------|-------------|
| `deploy.yml` | Multiple input types | choice (required), boolean, string (optional) |
| `no-name.yaml` | Workflow without name | Single string input with default |
| `number-input.yml` | Number type inputs | Three number inputs with varying requirements |
| `environment-type.yml` | Environment type | Environment type (future feature) + boolean |

### Complex Input Scenarios

| Workflow | Purpose | Key Features |
|----------|---------|--------------|
| `all-required.yml` | All required inputs | Tests validation of required fields |
| `many-inputs.yml` | Scrolling and pagination | 10 inputs to test form scrolling |
| `boolean-variations.yml` | Boolean edge cases | Different default formats, required vs optional |
| `edge-cases.yml` | Special characters and formatting | Multi-line descriptions, special chars, missing fields |
| `mixed-triggers.yml` | Multiple trigger types | workflow_dispatch + push + PR |
| `release.yml` | Real-world complex workflow | 6 inputs with varied types, realistic scenario |

## Testing Scenarios

### Discovery & Filtering
- **Expected dispatchable**: simple-dispatch, deploy, no-name, number-input, environment-type, all-required, many-inputs, boolean-variations, edge-cases, mixed-triggers, release, minimal
- **Expected filtered out**: ci, not-dispatchable

### Input Type Handling

**String inputs:**
- With defaults: `deploy.yml` (version), `edge-cases.yml` (various)
- Without defaults: `all-required.yml` (admin_email)
- Empty defaults: `edge-cases.yml` (empty_default)

**Boolean inputs:**
- Default true: `boolean-variations.yml` (bool_true_string)
- Default false: `boolean-variations.yml` (bool_false_string)
- No default: `boolean-variations.yml` (bool_no_default)
- Required: `boolean-variations.yml` (bool_required)

**Choice inputs:**
- With default: `deploy.yml` (environment=staging)
- Without default: `all-required.yml` (service)
- Many options: `many-inputs.yml` (input_08)

**Number inputs:**
- Required: `number-input.yml` (parallel_jobs)
- Optional with defaults: `number-input.yml` (timeout, max_retries)

**Environment inputs:**
- Required: `environment-type.yml` (target)

### Edge Cases

**Missing fields:**
- Missing name: `minimal.yml`, `no-name.yaml`
- Missing description: `edge-cases.yml` (no_description)
- Missing default: multiple workflows

**Special formatting:**
- Multi-line descriptions: `edge-cases.yml` (multiline_desc)
- Special characters: `edge-cases.yml` (special_chars)
- Hyphenated keys: `edge-cases.yml` (hyphenated-input)
- Uppercase keys: `edge-cases.yml` (UPPERCASE_INPUT)

**Validation:**
- All required fields: `all-required.yml`
- Mixed required/optional: `deploy.yml`, `number-input.yml`

### UI/UX Testing

**Form scrolling:**
- Use `many-inputs.yml` (10 inputs)

**Long descriptions:**
- Use `edge-cases.yml` (multiline_desc)

**Realistic workflows:**
- `release.yml`: Complex release process with 6 inputs
- `deploy.yml`: Common deployment scenario

## Manual Testing Checklist

- [ ] Discovery finds all dispatchable workflows
- [ ] Fuzzy filtering works on workflow names
- [ ] String inputs display correctly with/without defaults
- [ ] Boolean inputs show as confirm dialogs
- [ ] Choice inputs show all options
- [ ] Number inputs validate numeric input
- [ ] Required field validation works
- [ ] Multi-line descriptions render correctly
- [ ] Special characters in defaults don't break parsing
- [ ] Form scrolls when many inputs present
- [ ] Missing name field uses filename fallback
- [ ] Non-dispatchable workflows filtered out
- [ ] Branch selection works
- [ ] Command preview shows correct gh CLI invocation
- [ ] Execution succeeds with all input types
