export type FormState = Record<string, unknown>

export type ValidationErrors = Record<string, string>

export type CommercialFormCatalog = {
  partiesByID: Record<string, Record<string, unknown>>
  vendorsByID: Record<string, Record<string, unknown>>
  invoicesByID: Record<string, Record<string, unknown>>
  billsByID: Record<string, Record<string, unknown>>
  paymentsByID: Record<string, Record<string, unknown>>
  productsByCode: Record<string, Record<string, unknown>>
  itemsByCode: Record<string, Record<string, unknown>>
  variantDimensionsByCode: Record<string, Record<string, unknown>>
  variantValuesByKey: Record<string, Record<string, unknown>>
  itemCategoriesByCode: Record<string, Record<string, unknown>>
  uomsByCode: Record<string, Record<string, unknown>>
  warehousesByCode: Record<string, Record<string, unknown>>
  workCentersByCode: Record<string, Record<string, unknown>>
  inventoryBatchesByID: Record<string, Record<string, unknown>>
  bomsByID: Record<string, Record<string, unknown>>
  bomVersionsByID: Record<string, Record<string, unknown>>
  taxCodesByCode: Record<string, Record<string, unknown>>
  taxProfilesByCode: Record<string, Record<string, unknown>>
  priceListsByCode: Record<string, Record<string, unknown>>
  priceListItemsByKey: Record<string, Record<string, unknown>>
  paymentMethodsByCode: Record<string, Record<string, unknown>>
}

export const emptyCommercialFormCatalog = (): CommercialFormCatalog => ({
  partiesByID: {},
  vendorsByID: {},
  invoicesByID: {},
  billsByID: {},
  paymentsByID: {},
  productsByCode: {},
  itemsByCode: {},
  variantDimensionsByCode: {},
  variantValuesByKey: {},
  itemCategoriesByCode: {},
  uomsByCode: {},
  warehousesByCode: {},
  workCentersByCode: {},
  inventoryBatchesByID: {},
  bomsByID: {},
  bomVersionsByID: {},
  taxCodesByCode: {},
  taxProfilesByCode: {},
  priceListsByCode: {},
  priceListItemsByKey: {},
  paymentMethodsByCode: {},
})
