BEGIN;

CREATE INDEX IF NOT EXISTS idx_document_records_type_status
    ON document_records (document_type, status);

CREATE INDEX IF NOT EXISTS idx_document_records_org_loc_status
    ON document_records (organization_id, location_id, status);

CREATE INDEX IF NOT EXISTS idx_document_records_updated_at
    ON document_records (updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_document_records_payload_json_gin
    ON document_records USING GIN (payload_json);

CREATE INDEX IF NOT EXISTS idx_model_records_model_updated_at
    ON model_records (model_key, updated_at DESC);

CREATE INDEX IF NOT EXISTS idx_model_records_values_json_gin
    ON model_records USING GIN (values_json);

CREATE INDEX IF NOT EXISTS idx_search_document_summaries_type_status
    ON search_document_summaries (document_type, status);

CREATE INDEX IF NOT EXISTS idx_search_document_summaries_org_loc
    ON search_document_summaries (organization_id, location_id);

CREATE INDEX IF NOT EXISTS idx_search_document_summaries_updated_at
    ON search_document_summaries (updated_at DESC);

COMMIT;
