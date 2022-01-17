package coretools

// NamespaceDeletionError indicates that a namespace could not be created
// because a previously-created namespace with the same name is pending
// deletion. This occurs when you delete and recreate a chartassignment. It is
// transient, but may last seconds or minutes if the namespace contains
// resources that are slow to delete.
type NamespaceDeletionError struct {
	msg string
}

func (e *NamespaceDeletionError) Error() string { return e.msg }

// NewNamespaceDeletionError returns a new instance of the error
func NewNamespaceDeletionError(msg string) *NamespaceDeletionError {
	return &NamespaceDeletionError{
		msg: msg,
	}
}

// MissingServiceAccountError indicates that the default ServiceAccount has not
// yet been created, and that the chart should not be updated to avoid creating
// pods before the ImagePullSecrets have been applied.
type MissingServiceAccountError struct {
	msg string
}

func (e *MissingServiceAccountError) Error() string { return e.msg }

// NewMissingServiceAccountError returns a new instance of the error
func NewMissingServiceAccountError(msg string) *MissingServiceAccountError {
	return &MissingServiceAccountError{
		msg: msg,
	}
}
