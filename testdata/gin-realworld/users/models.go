package users

// UserModel is the persisted user record.
type UserModel struct {
	ID       uint
	Username string
	Email    string
	Bio      string
	Image    string
}

// FindOneUser stands in for the database lookup.
func FindOneUser(username string) (UserModel, error) {
	return UserModel{Username: username}, nil
}
