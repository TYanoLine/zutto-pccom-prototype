package replymodel

// Projection is a host-program-specific projection of one semantic response.
//
// ParentID is non-zero only when the host software natively stores/exposes the
// response as a child of the source article. Subject is the response article's
// host-native subject; it may be empty for append-style systems that have no
// independent response subject.
type Projection struct {
	ParentID int64
	Subject  string
}
