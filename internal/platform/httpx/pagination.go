package httpx

import "orbyte/internal/platform/model"

func normalizePagination(page, pageSize int) (int, int) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = model.DefaultPageSize
	}
	return page, pageSize
}

func paginateSlice[T any](items []T, page, pageSize int) ([]T, int) {
	page, pageSize = normalizePagination(page, pageSize)
	total := len(items)
	start := (page - 1) * pageSize
	if start >= total {
		return []T{}, total
	}
	end := start + pageSize
	if end > total {
		end = total
	}
	return items[start:end], total
}
