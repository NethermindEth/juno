package rpcv10

// Used to set the private `includeLastUpdateBlock` field to a given value.
func (st *StorageAtResult) IncludeLastUpdateBlock(val *bool) bool {
	if val == nil {
		return st.includeLastUpdateBlock
	}
	st.includeLastUpdateBlock = *val
	return *val
}
