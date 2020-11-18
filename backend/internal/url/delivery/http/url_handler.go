package http

import (
	"context"
	"fmt"
	"net/http"
	"regexp"

	"github.com/dgrijalva/jwt-go"
	ut "github.com/go-playground/universal-translator"
	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/label"
	"go.uber.org/zap"

	"bitbucket.org/dbproject_ivt/db/backend/internal/models"
	"bitbucket.org/dbproject_ivt/db/backend/internal/platform/auth"
	"bitbucket.org/dbproject_ivt/db/backend/internal/platform/web"
	"bitbucket.org/dbproject_ivt/db/backend/internal/url"
)

// URLHandler represent the http handler for url
type URLHandler struct {
	URLUsecase    url.Usecase
	Authenticator *auth.Authenticator
	Validator     *web.AppValidator
	logger        *zap.Logger
	Tracer        trace.Tracer
}

// NewURLHandler will initialize the url/ resources endpoint
func NewURLHandler(e *echo.Echo, us url.Usecase, authenticator *auth.Authenticator, v *web.AppValidator, logger *zap.Logger, tracer trace.Tracer) error {
	handler := &URLHandler{
		URLUsecase:    us,
		Authenticator: authenticator,
		Validator:     v,
		logger:        logger,
		Tracer:        tracer,
	}

	err := handler.RegisterValidation()
	if err != nil {
		return err
	}

	e.POST("/v1/url/create", handler.Store)
	e.POST("/v1/user/url/create", handler.StoreUserURL, middleware.JWTWithConfig(authenticator.JWTConfig))
	e.GET("/:id", handler.Redirect)
	e.GET("/v1/url/:id", handler.GetByID)
	e.DELETE("/v1/url/:id", handler.Delete, middleware.JWTWithConfig(authenticator.JWTConfig))
	e.PUT("/v1/url", handler.Update, middleware.JWTWithConfig(authenticator.JWTConfig))

	return nil
}

// RegisterValidation will initialize validation for url handler
func (uh *URLHandler) RegisterValidation() error {
	err := uh.Validator.V.RegisterValidation("linkid", checkURL)
	if err != nil {
		return err
	}

	err = uh.Validator.V.RegisterTranslation("linkid", uh.Validator.Translator, func(ut ut.Translator) error {
		return ut.Add("linkid", "{0} must contain only a-z, A-Z, 0-9, _, - characters", true)
	}, func(ut ut.Translator, fe validator.FieldError) string {
		t, _ := ut.T("linkid", fe.Field())
		return t
	})
	if err != nil {
		return err
	}

	return nil
}

func checkURL(fl validator.FieldLevel) bool {
	r := regexp.MustCompile(`^[A-Za-z0-9_-]+$`)
	return r.MatchString(fl.Field().String())
}

// Redirect will redirect to link by given id
func (uh *URLHandler) Redirect(c echo.Context) error {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http Redirect",
	)
	defer span.End()

	u, err := uh.getByID(c)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	if u != nil {
		return c.Redirect(http.StatusMovedPermanently, u.Link)
	}
	return nil
}

// GetByID will get url by given id
func (uh *URLHandler) GetByID(c echo.Context) error {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http GetByID",
	)
	defer span.End()

	u, err := uh.getByID(c)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return err
	}

	if u != nil {
		return c.JSON(http.StatusOK, u)
	}
	return nil
}

func (uh *URLHandler) getByID(c echo.Context) (*models.URL, error) {
	id := c.Param("id")

	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http getByID",
	)
	defer span.End()

	err := uh.Validator.V.Var(id, "required,linkid,max=20")
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		fields := err.(validator.ValidationErrors).Translate(uh.Validator.Translator)
		return nil, c.JSON(http.StatusBadRequest, web.ResponseError{Error: "validation error", Fields: fields})
	}

	u, err := uh.URLUsecase.GetByID(ctx, id)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return nil, c.JSON(web.GetStatusCode(err, uh.logger), web.ResponseError{Error: err.Error()})
	}
	span.SetAttributes(
		label.String("urlid", id),
	)

	return u, nil
}

// Store will store the URL by given request body
func (uh *URLHandler) Store(c echo.Context) error {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http Store",
	)
	defer span.End()

	u := new(models.CreateURL)
	return uh.storeURL(ctx, c, u)
}

// StoreUserURL will store the URL of authenticated user by given request body
func (uh *URLHandler) StoreUserURL(c echo.Context) error {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http StoreUserURL",
	)
	defer span.End()

	u := new(models.CreateURL)
	token, ok := c.Get("user").(*jwt.Token)
	if !ok || token == nil {
		span.RecordError(ctx, web.ErrForbidden, trace.WithErrorStatus(codes.Error))
		return c.JSON(http.StatusForbidden, web.ResponseError{Error: web.ErrForbidden.Error()})
	}
	user, ok := token.Claims.(*auth.Claims)
	if !ok {
		span.RecordError(ctx, web.ErrInternalServerError, trace.WithErrorStatus(codes.Error))
		return fmt.Errorf("%w can't convert jwt.Claims to auth.Claims", web.ErrInternalServerError)
	}

	u.UserID = user.Subject

	span.SetAttributes(
		label.String("userid", user.Id),
	)

	return uh.storeURL(ctx, c, u)
}

func (uh *URLHandler) storeURL(ctx context.Context, c echo.Context, u *models.CreateURL) error {
	ctx, span := uh.Tracer.Start(
		ctx,
		"http storeURL",
	)
	defer span.End()

	if err := c.Bind(u); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return c.JSON(http.StatusBadRequest, web.ResponseError{Error: err.Error()})
	}

	if err := c.Validate(u); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		fields := err.(validator.ValidationErrors).Translate(uh.Validator.Translator)
		return c.JSON(http.StatusBadRequest, web.ResponseError{Error: "validation error", Fields: fields})
	}

	result, err := uh.URLUsecase.Store(ctx, *u)
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return c.JSON(web.GetStatusCode(err, uh.logger), web.ResponseError{Error: err.Error()})
	}

	span.SetAttributes(
		label.String("urlid", result.ID),
	)

	return c.JSON(http.StatusCreated, result)
}

// Delete will delete URL by given id
func (uh *URLHandler) Delete(c echo.Context) error {
	id := c.Param("id")

	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http Delete",
	)
	defer span.End()

	err := uh.Validator.V.Var(id, "required,linkid,max=20")
	if err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		fields := err.(validator.ValidationErrors).Translate(uh.Validator.Translator)
		return c.JSON(http.StatusBadRequest, web.ResponseError{Error: "validation error", Fields: fields})
	}

	token, ok := c.Get("user").(*jwt.Token)
	if !ok || token == nil {
		span.RecordError(ctx, web.ErrForbidden, trace.WithErrorStatus(codes.Error))
		return c.JSON(http.StatusForbidden, web.ResponseError{Error: web.ErrForbidden.Error()})
	}
	user, ok := token.Claims.(*auth.Claims)
	if !ok {
		span.RecordError(ctx, web.ErrInternalServerError, trace.WithErrorStatus(codes.Error))
		return fmt.Errorf("%w can't convert jwt.Claims to auth.Claims", web.ErrInternalServerError)
	}

	if err = uh.URLUsecase.Delete(ctx, id, *user); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return c.JSON(web.GetStatusCode(err, uh.logger), web.ResponseError{Error: err.Error()})
	}

	span.SetAttributes(
		label.String("userid", user.Id),
		label.String("urlid", id),
	)

	return c.JSON(http.StatusNoContent, nil)
}

// Update will update the URL by given request body
func (uh *URLHandler) Update(c echo.Context) error {
	ctx := c.Request().Context()
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, span := uh.Tracer.Start(
		ctx,
		"http Update",
	)
	defer span.End()

	u := new(models.UpdateURL)
	if err := c.Bind(u); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return c.JSON(http.StatusBadRequest, web.ResponseError{Error: err.Error()})
	}

	if err := c.Validate(u); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		fields := err.(validator.ValidationErrors).Translate(uh.Validator.Translator)
		return c.JSON(http.StatusBadRequest, web.ResponseError{Error: "validation error", Fields: fields})
	}

	token, ok := c.Get("user").(*jwt.Token)
	if !ok || token == nil {
		span.RecordError(ctx, web.ErrForbidden, trace.WithErrorStatus(codes.Error))
		return c.JSON(http.StatusForbidden, web.ResponseError{Error: web.ErrForbidden.Error()})
	}
	user, ok := token.Claims.(*auth.Claims)
	if !ok {
		span.RecordError(ctx, web.ErrInternalServerError, trace.WithErrorStatus(codes.Error))
		return fmt.Errorf("%w can't convert jwt.Claims to auth.Claims", web.ErrInternalServerError)
	}

	if err := uh.URLUsecase.Update(ctx, *u, *user); err != nil {
		span.RecordError(ctx, err, trace.WithErrorStatus(codes.Error))
		return c.JSON(web.GetStatusCode(err, uh.logger), web.ResponseError{Error: err.Error()})
	}

	span.SetAttributes(
		label.String("userid", user.Id),
		label.String("urlid", u.ID),
	)

	return c.JSON(http.StatusNoContent, nil)
}
