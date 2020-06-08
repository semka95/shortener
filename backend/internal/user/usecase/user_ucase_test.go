package usecase_test

import (
	"context"
	"testing"
	"time"

	"bitbucket.org/dbproject_ivt/db/backend/internal/models"
	"bitbucket.org/dbproject_ivt/db/backend/internal/platform/auth"
	"bitbucket.org/dbproject_ivt/db/backend/internal/platform/web"
	"bitbucket.org/dbproject_ivt/db/backend/internal/tests"
	"bitbucket.org/dbproject_ivt/db/backend/internal/user/mocks"
	"bitbucket.org/dbproject_ivt/db/backend/internal/user/usecase"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/assert"
	"golang.org/x/crypto/bcrypt"
)

func TestUserUsecase_GetByID(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tUser := tests.NewUser()

	repository := mocks.NewMockRepository(controller)
	uc := usecase.NewUserUsecase(repository, 10*time.Second)

	t.Run("get not valid id", func(t *testing.T) {
		result, err := uc.GetByID(context.Background(), "not valid id")
		assert.Error(t, err, web.ErrBadParamInput)
		assert.Nil(t, result)
	})

	t.Run("get not existed user", func(t *testing.T) {
		repository.EXPECT().GetByID(gomock.Any(), tUser.ID).Return(nil, web.ErrNotFound)
		result, err := uc.GetByID(context.Background(), tUser.ID.Hex())
		assert.Error(t, err, web.ErrNotFound)
		assert.Nil(t, result)
	})

	t.Run("get user success", func(t *testing.T) {
		repository.EXPECT().GetByID(gomock.Any(), tUser.ID).Return(tUser, nil)
		result, err := uc.GetByID(context.Background(), tUser.ID.Hex())
		assert.NoError(t, err)
		assert.EqualValues(t, tUser, result)
	})
}

func TestUserUsecase_Update(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tUser := tests.NewUser()
	tUpdateUser := tests.NewUpdateUser()

	repository := mocks.NewMockRepository(controller)
	uc := usecase.NewUserUsecase(repository, 10*time.Second)
	claims := auth.NewClaims("507f191e810c19729de860ea", []string{auth.RoleUser}, time.Now(), time.Minute)

	t.Run("update not existed user", func(t *testing.T) {
		repository.EXPECT().GetByID(gomock.Any(), tUpdateUser.ID).Return(nil, web.ErrNotFound)
		err := uc.Update(context.Background(), tUpdateUser, *claims)
		assert.Error(t, err, web.ErrNotFound)
	})

	t.Run("update user success", func(t *testing.T) {
		repository.EXPECT().GetByID(gomock.Any(), tUpdateUser.ID).Return(tUser, nil)
		repository.EXPECT().Update(gomock.Any(), tUser).Return(nil)

		err := uc.Update(context.Background(), tUpdateUser, *claims)
		assert.NoError(t, err)

		errP := bcrypt.CompareHashAndPassword([]byte(tUser.HashedPassword), []byte(*tUpdateUser.Password))
		assert.NoError(t, errP)

		assert.Equal(t, *tUpdateUser.FullName, tUser.FullName)
		assert.Equal(t, *tUpdateUser.Email, tUser.Email)
	})

	t.Run("update all fields are empty", func(t *testing.T) {
		tUser = tests.NewUser()
		tUserOld := &models.User{
			ID:             tUser.ID,
			FullName:       tUser.FullName,
			Email:          tUser.Email,
			Roles:          tUser.Roles,
			HashedPassword: tUser.HashedPassword,
			CreatedAt:      tUser.CreatedAt,
			UpdatedAt:      tUser.UpdatedAt,
		}
		tUpdateUser.Email = nil
		tUpdateUser.FullName = nil
		tUpdateUser.Password = nil

		repository.EXPECT().GetByID(gomock.Any(), tUpdateUser.ID).Return(tUser, nil)
		repository.EXPECT().Update(gomock.Any(), tUser).Return(nil)

		err := uc.Update(context.Background(), tUpdateUser, *claims)
		assert.NoError(t, err)

		assert.EqualValues(t, tUserOld, tUser)
	})

	t.Run("update user wrong user", func(t *testing.T) {
		claims.Subject = "wrong user"
		repository.EXPECT().GetByID(gomock.Any(), tUpdateUser.ID).Return(tUser, nil)

		err := uc.Update(context.Background(), tUpdateUser, *claims)
		assert.Error(t, web.ErrForbidden, err)
	})

	t.Run("update user success by wrong user, but admin", func(t *testing.T) {
		claims.Subject = "wrong user"
		claims.Roles = append(claims.Roles, auth.RoleAdmin)
		repository.EXPECT().GetByID(gomock.Any(), tUpdateUser.ID).Return(tUser, nil)
		repository.EXPECT().Update(gomock.Any(), gomock.Any()).Return(nil)

		err := uc.Update(context.Background(), tUpdateUser, *claims)
		assert.NoError(t, err)
	})
}

func TestUserUsecase_Create(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tCreateUser := tests.NewCreateUser()

	repository := mocks.NewMockRepository(controller)
	uc := usecase.NewUserUsecase(repository, 10*time.Second)

	t.Run("create user error", func(t *testing.T) {
		repository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(web.ErrInternalServerError)
		result, err := uc.Create(context.Background(), tCreateUser)
		assert.Error(t, err, web.ErrInternalServerError)
		assert.Empty(t, result)
	})

	t.Run("create user success", func(t *testing.T) {
		repository.EXPECT().Create(gomock.Any(), gomock.Any()).Return(nil)
		result, err := uc.Create(context.Background(), tCreateUser)
		assert.NoError(t, err)

		errP := bcrypt.CompareHashAndPassword([]byte(result.HashedPassword), []byte(tCreateUser.Password))
		assert.NoError(t, errP)

		assert.Equal(t, tCreateUser.Email, result.Email)
		assert.Equal(t, tCreateUser.FullName, result.FullName)
	})
}

func TestUserUsecase_Delete(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tUser := tests.NewUser()

	repository := mocks.NewMockRepository(controller)
	uc := usecase.NewUserUsecase(repository, 10*time.Second)

	t.Run("delete not valid id", func(t *testing.T) {
		err := uc.Delete(context.Background(), "not valid id")
		assert.Error(t, err, web.ErrBadParamInput)
	})

	t.Run("delete not existed user", func(t *testing.T) {
		repository.EXPECT().Delete(gomock.Any(), tUser.ID).Return(web.ErrNoAffected)
		err := uc.Delete(context.Background(), tUser.ID.Hex())
		assert.Error(t, err, web.ErrNoAffected)
	})

	t.Run("delete success", func(t *testing.T) {
		repository.EXPECT().Delete(gomock.Any(), tUser.ID).Return(nil)
		err := uc.Delete(context.Background(), tUser.ID.Hex())
		assert.NoError(t, err)
	})
}

func TestUserUsecase_Authenticate(t *testing.T) {
	controller := gomock.NewController(t)
	defer controller.Finish()

	tUser := tests.NewUser()
	now := time.Now()
	password := "password"

	repository := mocks.NewMockRepository(controller)
	uc := usecase.NewUserUsecase(repository, 10*time.Second)

	t.Run("auth user not found", func(t *testing.T) {
		repository.EXPECT().GetByEmail(gomock.Any(), tUser.Email).Return(nil, web.ErrNotFound)
		result, err := uc.Authenticate(context.Background(), now, tUser.Email, password)
		assert.Error(t, err, web.ErrAuthenticationFailure)
		assert.Nil(t, result)
	})

	t.Run("auth incorrect password", func(t *testing.T) {
		repository.EXPECT().GetByEmail(gomock.Any(), tUser.Email).Return(tUser, nil)
		result, err := uc.Authenticate(context.Background(), now, tUser.Email, "incorrect_pwd")
		assert.Error(t, err, web.ErrAuthenticationFailure)
		assert.Nil(t, result)
	})

	t.Run("auth user success", func(t *testing.T) {
		repository.EXPECT().GetByEmail(gomock.Any(), tUser.Email).Return(tUser, nil)
		result, err := uc.Authenticate(context.Background(), now, tUser.Email, password)
		assert.NoError(t, err)
		assert.Equal(t, result.Roles[0], auth.RoleUser)
		assert.Equal(t, result.Subject, tUser.ID.Hex())
		assert.Equal(t, result.IssuedAt, now.Unix())
	})
}
