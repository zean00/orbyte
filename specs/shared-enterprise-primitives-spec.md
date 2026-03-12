# Shared Enterprise Primitives Specification

## 1. Purpose

This document defines the shared enterprise primitives used by the domain-agnostic platform kernel.

Its purpose is to establish a stable set of reusable value objects and supporting reference concepts that can be shared across domain packs such as clinic, ERP, OMS, and POS.

These primitives are not business modules. They are foundational building blocks used by canonical types, workflows, policies, integrations, and projections.

---

## 2. Goals

- define reusable cross-domain value objects
- reduce duplicate modeling of common business concepts
- standardize validation and serialization of shared primitive types
- support consistent APIs, storage, and integrations
- keep primitives small, stable, and domain-neutral

---

## 3. Non-Goals

This document does not define:

- full accounting logic
- full inventory logic
- domain-specific pricing engines
- clinical coding systems or other industry-specific terminology sets
- UI input widgets or display formatting details for each frontend

---

## 4. Design Principles

1. **Primitives are reusable, not domain-owned**  
   A primitive must be valid in multiple domains.

2. **Value objects before modules**  
   Shared primitives should be modeled as small, composable value objects rather than large subsystems.

3. **Stable semantics**  
   Once introduced, a primitive should have clear semantics that do not change by domain.

4. **Explicit units and context**  
   Ambiguous values such as money, quantity, and time must always carry enough context to be interpreted safely.

5. **No hidden locale assumptions**  
   Formatting, display, and localization must not alter canonical values.

6. **Validation at boundaries**  
   Primitives should have standard validation rules for API, persistence, and integration boundaries.

---

## 5. Primitive Families

The shared primitives are grouped into these families:

- identity and reference primitives
- money and commercial primitives
- quantity and measurement primitives
- address and contact primitives
- temporal primitives
- classification and code primitives
- status and range primitives

---

## 6. Identity and Reference Primitives

### 6.1 `Identifier`

Represents a typed identifier value.

Purpose:

- capture internal or external identifiers consistently
- support validation and normalization
- preserve identifier type and issuing context

Recommended fields:

- `identifier_type`
- `value`
- `issuer` (nullable)
- `namespace` (nullable)
- `status` (nullable)

Rules:

- preserve the canonical stored value separately from display formatting
- normalization rules must be type-specific
- identifier comparison rules must be explicit and not inferred globally

### 6.2 `CodeReference`

Represents a coded value from an internal or external classification system.

Recommended fields:

- `code_system`
- `code`
- `display` (nullable)
- `version` (nullable)

Rules:

- code system must always be explicit
- display text is informative and must not be treated as canonical meaning

### 6.3 `ReferenceKey`

Represents a lightweight stable key used to point to configurable reference data.

Recommended fields:

- `reference_type`
- `reference_key`
- `version` (nullable)

---

## 7. Money and Commercial Primitives

### 7.1 `Currency`

Represents a currency definition.

Recommended fields:

- `currency_code`
- `minor_unit_scale`
- `display_symbol` (nullable)

Rules:

- currency code should use a stable standard such as ISO-like codes where available
- scale must be explicit for storage and arithmetic

### 7.2 `Money`

Represents a monetary amount in a specific currency.

Recommended fields:

- `amount`
- `currency_code`

Rules:

- never store money without currency
- arithmetic must only occur between values of the same currency unless an explicit conversion policy applies
- canonical storage should avoid binary floating-point ambiguity

### 7.3 `Price`

Represents a priced value that may include commercial context.

Recommended fields:

- `base_amount`
- `currency_code`
- `price_type`
- `effective_from` (nullable)
- `effective_to` (nullable)

### 7.4 `TaxComponent`

Represents a tax element applied to a commercial amount.

Recommended fields:

- `tax_type`
- `rate` (nullable)
- `amount` (nullable)
- `jurisdiction` (nullable)
- `code_ref` (nullable)

Rules:

- tax is a composable primitive, not a full tax engine
- tax rules remain policy-driven and domain-defined

### 7.5 `DiscountComponent`

Represents a discount adjustment component.

Recommended fields:

- `discount_type`
- `amount` (nullable)
- `rate` (nullable)
- `reason_code` (nullable)

---

## 8. Quantity and Measurement Primitives

### 8.1 `UnitOfMeasure`

Represents a named measurement unit.

Recommended fields:

- `uom_code`
- `dimension`
- `display_name`
- `precision_scale`

Examples:

- item count
- weight
- volume
- length
- time unit
- dosage unit

### 8.2 `Quantity`

Represents a numeric amount paired with a unit.

Recommended fields:

- `value`
- `uom_code`

Rules:

- quantity must always have an explicit unit when the value is unit-dependent
- conversion must be explicit through allowed unit relationships

### 8.3 `Measurement`

Represents an observed value with optional method or context metadata.

Recommended fields:

- `value`
- `uom_code`
- `measurement_type`
- `observed_at` (nullable)
- `method` (nullable)

This primitive is useful for clinic, logistics, and quality contexts without making it domain-specific.

---

## 9. Address and Contact Primitives

### 9.1 `Address`

Represents a structured physical or mailing address.

Recommended fields:

- `address_line_1`
- `address_line_2` (nullable)
- `locality`
- `subregion` (nullable)
- `region` (nullable)
- `postal_code` (nullable)
- `country_code`
- `address_type` (nullable)

Rules:

- preserve structured fields where possible
- formatted display address may be derived but is not the canonical stored form

### 9.2 `ContactMethod`

Represents a typed communication method.

Recommended fields:

- `contact_type`
- `value`
- `usage_type` (nullable)
- `is_primary` (nullable)
- `status` (nullable)

Examples:

- phone
- mobile
- email
- messaging handle

### 9.3 `PersonName`

Represents a structured personal name.

Recommended fields:

- `full_name`
- `given_name` (nullable)
- `middle_name` (nullable)
- `family_name` (nullable)
- `prefix` (nullable)
- `suffix` (nullable)

Rules:

- keep full displayable name available
- structured components are optional where cultural name rules differ

---

## 10. Temporal Primitives

### 10.1 `Instant`

Represents a specific point in time.

Rules:

- should be stored in a canonical timezone-safe form
- display localization is a presentation concern

### 10.2 `DateValue`

Represents a date without time-of-day semantics.

Used for:

- business dates
- birth dates
- settlement dates
- posting dates

### 10.3 `TimeRange`

Represents a start and end temporal boundary.

Recommended fields:

- `start_at`
- `end_at`

Rules:

- end must not precede start
- boundary inclusivity rules must be explicit at the calling context

### 10.4 `ScheduleWindow`

Represents an operating or availability window.

Recommended fields:

- `day_of_week` or `calendar_ref`
- `start_time`
- `end_time`
- `timezone`
- `effective_range` (nullable)

### 10.5 `DurationValue`

Represents a duration amount with unit.

Recommended fields:

- `value`
- `uom_code`

---

## 11. Classification and Catalog Primitives

### 11.1 `CategoryRef`

Represents a category or classification link.

Recommended fields:

- `category_type`
- `category_key`
- `display` (nullable)

### 11.2 `CatalogRef`

Represents a stable reference to a catalog item or service definition.

Recommended fields:

- `catalog_type`
- `catalog_id`
- `code` (nullable)
- `display_name` (nullable)

This primitive does not define catalog ownership; it only standardizes cross-module references.

---

## 12. Status and Range Primitives

### 12.1 `StatusValue`

Represents a normalized status key with optional metadata.

Recommended fields:

- `status_key`
- `status_reason` (nullable)
- `effective_at` (nullable)

### 12.2 `NumericRange`

Represents a bounded numeric interval.

Recommended fields:

- `min_value` (nullable)
- `max_value` (nullable)
- `uom_code` (nullable)

### 12.3 `PriorityValue`

Represents a normalized priority classification.

Recommended fields:

- `priority_key`
- `weight` (nullable)

---

## 13. Primitive Usage Rules

### 13.1 Composition Rules

Primitives should be used by composition inside canonical types and domain models.

Examples:

- `Party` uses `PersonName`, `Address`, `ContactMethod`, `Identifier`
- `DocumentLine` uses `Quantity`, `Money`, `TaxComponent`, `CatalogRef`
- `Location` uses `Address`, `ScheduleWindow`
- `Policy` may use `NumericRange`, `Money`, `CategoryRef`

### 13.2 Extension Rules

- domains may add domain-specific wrappers around shared primitives
- domains may not redefine core primitive semantics
- integration adapters may map provider-specific formats into these primitives

### 13.3 Serialization Rules

- serialized representations must preserve type meaning and context
- display-only formatting must not replace canonical primitive fields
- nullability must be explicit and meaningful

---

## 14. Validation Rules

Each primitive family should have standard validation rules.

Examples:

- `Money.amount` must respect configured precision
- `Quantity` must have a valid `uom_code`
- `Address.country_code` must be present when address is used for official context
- `TimeRange.end_at` must not be earlier than `start_at`
- `Identifier.value` must not be empty once created
- `Currency.minor_unit_scale` must be non-negative

Validation rules should be reusable across API, persistence, workflow, and integration layers.

---

## 15. Storage Guidance

The default storage approach depends on usage frequency and query needs.

Recommended rules:

- highly reused primitives such as money, quantity, identifiers, and dates may be flattened into parent records for indexing efficiency
- repeatable primitives such as contact methods and addresses may use child tables or structured JSON depending on query needs
- provider-specific raw representations should not replace canonical primitive storage

Examples:

- `Money` stored as decimal-compatible amount + currency code columns
- `Quantity` stored as numeric value + `uom_code`
- `Identifier` stored in normalized reference tables where cross-record lookup is needed

---

## 16. API Guidance

APIs should expose shared primitives in stable shapes.

Examples:

- money values must always include currency
- quantity values must always include unit where required
- addresses should be structured rather than sent only as one formatted string
- identifiers should carry type context where ambiguity exists

Avoid:

- raw numbers with implied currency
- free-text units without controlled code references
- overloaded generic strings for identifiers or statuses

---

## 17. Integration Guidance

Shared primitives are the preferred normalization layer between internal canonical models and external adapters.

Examples:

- external customer code -> `Identifier`
- remote monetary total -> `Money`
- shipment weight -> `Quantity`
- provider tax line -> `TaxComponent`
- remote facility address -> `Address`

This makes connector logic simpler and keeps provider-specific formats from leaking into canonical models.

---

## 18. Governance Rules

- a primitive may enter the shared platform only if it is useful across multiple domains
- primitive semantics must be stable and documented
- any primitive change that affects serialization or meaning requires compatibility review
- domains must prefer existing primitives before inventing new near-duplicates
- provider-specific formats must be normalized into shared primitives at integration boundaries

---

## 19. Recommended Initial Primitive Set

The recommended first wave of shared primitives is:

- `Identifier`
- `CodeReference`
- `Currency`
- `Money`
- `UnitOfMeasure`
- `Quantity`
- `Address`
- `ContactMethod`
- `PersonName`
- `Instant`
- `DateValue`
- `TimeRange`
- `ScheduleWindow`
- `CatalogRef`
- `StatusValue`
- `PriorityValue`

The following may be added in the next wave if reuse is confirmed:

- `TaxComponent`
- `DiscountComponent`
- `Measurement`
- `NumericRange`
- `CategoryRef`
- `DurationValue`

---

## 20. Final Summary

The shared enterprise primitives provide a stable value-object layer for the platform kernel.

They standardize recurring concepts such as:

- identifiers
- codes
- money
- quantity
- address
- contact methods
- names
- time ranges
- categories
- statuses

These primitives help keep the platform domain-agnostic, reduce duplication across domain packs, and improve consistency in storage, APIs, workflows, and integrations.
