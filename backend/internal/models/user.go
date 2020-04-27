package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// User represents the User model
type User struct {
	ID        primitive.ObjectID `json:"id,omitempty" bson:"_id"`
	FullName  string             `json:"full_name,omitempty" bson:"full_name,omitempty" validate:"max=30"`
	Email     string             `json:"email,omitempty" bson:"email,omitempty" validate:"email"`
	Password  string             `json:"password,omitempty" bson:"password,omitempty" validate:"min=8,max=30"`
	CreatedAt time.Time          `json:"created_at,omitempty" bson:"created_at"`
	UpdatedAt time.Time          `json:"updated_at" bson:"updated_at"`
}

// NewUser creates instance of User model
func NewUser() *User {
	id, _ := primitive.ObjectIDFromHex("507f191e810c19729de860ea")
	return &User{
		ID:        id,
		FullName:  "John Doe",
		Email:     "test@example.com",
		Password:  "",
		CreatedAt: time.Now().Truncate(time.Millisecond).UTC(),
		UpdatedAt: time.Now().Truncate(time.Millisecond).UTC(),
	}
}

// Sanitize clears user's password
func (u *User) Sanitize() {
	u.Password = ""
}