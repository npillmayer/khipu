package contracts

// AtomicID identifies one opaque inline occurrence. Zero is invalid. IDs are
// unique within a paragraph, not global object or renderer handles.
type AtomicID uint64

// AtomicAnchor attaches an occurrence to the first UTF-8 byte of a U+FFFC
// OBJECT REPLACEMENT CHARACTER in the original paragraph. Anchors are strictly
// ordered by paragraph-relative byte offset. Unattached U+FFFC remains text.
type AtomicAnchor struct {
	ID         AtomicID
	ByteOffset uint64
}
