# The `tests_readme.md` template

A worked sample of the test-description format for step 7 (the Tester agent). Copy the structure,
not the content: the domain here is invented (payments), and what matters is the split by layer, the
mock block, the table columns and the traceable path of an error.

The mandatory elements and what blocks the step are stated in `core/developer/steps/07-tester.md`;
they are not repeated here. This file shows what they look like when written out.

Names in the sample are placeholders. The concrete file names come from the profile's `layers[]`
(`test_file`), the error values from the stack, and the test-naming convention from the profile
overlay for step 7. A profile may add its own instance of this format in its overlay; the format
itself is fixed here.

Everything after the separator is what gets copied as a template.

---

# Tests of the payments module

## Contents

- [\<service layer\> — CreatePayment](#service-layer--createpayment)
- [\<service layer\> — GetPayment](#service-layer--getpayment)
- [\<error type\> — UserError](#error-type--usererror)
- [\<HTTP layer\> — CreatePayment](#http-layer--createpayment)
- [\<HTTP layer\> — GetPayment](#http-layer--getpayment)
- [\<integration suite\>](#integration-suite)

---

## \<service layer\> — CreatePayment

> Replaced by stubs: the repository (`fakeRepository`), the payment gateway (`fakeGateway`).
> Amount and currency validation lives in the service — the gateway is not called in those cases.

| Test | Scenario | Expected result |
|---|---|---|
| `CreatePayment_ZeroAmount` | amount = 0 | `ErrInvalidAmount`, the gateway is not called |
| `CreatePayment_NegativeAmount` | amount = -100 | `ErrInvalidAmount`, the gateway is not called |
| `CreatePayment_UnsupportedCurrency` | currency `XYZ` | `ErrUnsupportedCurrency` |
| `CreatePayment_DuplicateExternalID` | the repository reported a unique-constraint violation | `ErrDuplicate` |
| `CreatePayment_GatewayError` | the gateway returned a connection error | an internal error, the payment is not saved |
| `CreatePayment_DBError` | the insert returned a connection error | an internal error |
| `CreatePayment_Success` | a valid amount and currency | the payment is saved, its identifier is returned |
## \<service layer\> — GetPayment

> Replaced by a stub: the repository (`fakeRepository`).

| Test | Scenario | Expected result |
|---|---|---|
| `GetPayment_NotFound` | the query reported "no rows" | the domain `ErrNotFound` |
| `GetPayment_DBError` | the query returned a connection error | an internal error (not `ErrNotFound`) |
| `GetPayment_Success` | the query returned a row | a payment with its fields populated |
### Helper functions

> Pure functions with no dependencies; no mocks are needed.

| Test | Scenario | Expected result |
|---|---|---|
| `NormalizeCurrency_LowerCase` | `"rub"` | `"RUB"` |
| `NormalizeCurrency_Unknown` | `"xyz"` | an empty string, the "unsupported" flag |
## \<error type\> — UserError

> Unit tests of the `UserError` type. They check the wrapper contract the HTTP layer relies on.

| Test | Scenario | Expected result |
|---|---|---|
| `UserError_Message` | reading the message | the original message, unchanged |
| `UserError_TypeMatch` | matching a wrapped error against the type | the target is populated, the type is recognized |
| `UserError_Unwrap` | unwrapping | the wrapped error is returned |
## \<HTTP layer\> — CreatePayment

> The service is replaced by a stub. Helpers: the request-context builder, the message-field assert.

| Test | Scenario | Expected result |
|---|---|---|
| `HandlerCreatePayment_InvalidBody` | the request body does not parse | `400`, a non-empty `message` |
| `HandlerCreatePayment_InvalidAmount` | the service returned `ErrInvalidAmount` | `400`, `message`, `paramName = "amount"` |
| `HandlerCreatePayment_UnsupportedCurrency` | the service returned `ErrUnsupportedCurrency` | `400`, `message`, `paramName = "currency"` |
| `HandlerCreatePayment_Duplicate` | the service returned `ErrDuplicate` | `409`, no body |
| `HandlerCreatePayment_ServiceError` | the service returned an internal error | `500`, no body |
| `HandlerCreatePayment_Success` | the service returned an identifier | `200`, a body with `paymentId` |
## \<HTTP layer\> — GetPayment

> The service is replaced by a stub.

| Test | Scenario | Expected result |
|---|---|---|
| `HandlerGetPayment_NotFound` | the service returned `ErrNotFound` | `404`, no body |
| `HandlerGetPayment_ServiceError` | the service returned an internal error | `500`, no body |
| `HandlerGetPayment_Success` | the service returned a payment | `200`, a body with the payment |
## \<integration suite\>

> An integration test, run separately from the unit suite (`commands.test_integration`).
> It calls the real payment gateway.

| Test | Scenario | Expected result |
|---|---|---|
| `CreatePayment_RealGateway` | a real request against a test account | no error, the payment identifier is non-empty |

---

## What to watch for when copying

**The path of an error is traceable across the layers.** One and the same violation is visible at
three levels: "no rows" (the query) → `ErrNotFound` (the service) → `404` with no body (the HTTP
layer). If the tables do not let you follow what an error turns into, the description is incomplete.

**Response bodies follow the contract.** Which codes carry a body, and in what shape, is fixed by
the profile's `api_spec` role — not guessed. Write it out in `Expected result`: "no body" matters as
much as "a body with …".

**No rule identifiers in this file.** The document is committed and outlives the run that produced
it, while the rule registry is deleted with the run — so a `Rule` column, or a coverage section,
would leave later readers holding a number with nothing behind it. Which rule a test covers is
recorded in the run's coverage record under `.pipeline/workdir/coverage/`; here the scenario has to
be readable from the scenario.

**One rule — at least one test, usually two.** A check at the layer where the rule is enforced, plus
a check of the response code at the HTTP layer. A rule with no test blocks step 7 — the coverage
record is where that is checked.

**The document is organised by layer only.** No section, table or note is named after a changeset,
and none says the description is extended per changeset. A later changeset adds its rows to the
layer tables that already exist — the file always reads as the current state of the module's tests,
not as the order in which they were written.
