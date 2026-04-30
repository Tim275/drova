package grpc_handler

import (
	"context"
	"strings"
	"time"

	"drova/services/user-service/internal/auth"
	"drova/services/user-service/internal/domain"
	"drova/services/user-service/internal/service"
	pb "drova/shared/proto/user"

	"go.uber.org/zap"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type Handler struct {
	pb.UnimplementedUserServiceServer
	svc       *service.UserService
	auth      *auth.Authenticator
	blacklist domain.TokenBlacklist
	log       *zap.SugaredLogger
}

func New(svc *service.UserService, authenticator *auth.Authenticator, blacklist domain.TokenBlacklist, log *zap.SugaredLogger) *Handler {
	return &Handler{svc: svc, auth: authenticator, blacklist: blacklist, log: log}
}

func (h *Handler) Register(ctx context.Context, req *pb.RegisterRequest) (*pb.RegisterResponse, error) {
	if req.Email == "" || req.Password == "" || req.DisplayName == "" || req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "display_name, email, password and role required")
	}

	user, err := h.svc.Register(ctx, req.Email, req.Password, domain.Role(req.Role), req.DisplayName, req.Phone)
	if err != nil {
		if strings.Contains(err.Error(), "unique") {
			return nil, status.Error(codes.AlreadyExists, "email already registered")
		}
		h.log.Errorw("grpc register", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not create user")
	}

	return &pb.RegisterResponse{Id: user.ID, Email: user.Email, Role: string(user.Role)}, nil
}

func (h *Handler) Login(ctx context.Context, req *pb.LoginRequest) (*pb.LoginResponse, error) {
	if req.Email == "" || req.Password == "" {
		return nil, status.Error(codes.InvalidArgument, "email and password required")
	}

	user, err := h.svc.GetByEmail(ctx, req.Email)
	if err != nil || !user.IsActivated {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	match, err := user.Password.Matches(req.Password)
	if err != nil || !match {
		return nil, status.Error(codes.Unauthenticated, "invalid credentials")
	}

	token, err := h.auth.GenerateToken(user.ID, string(user.Role))
	if err != nil {
		h.log.Errorw("grpc login: generate token", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not generate token")
	}

	return &pb.LoginResponse{Token: token}, nil
}

func (h *Handler) Activate(ctx context.Context, req *pb.ActivateRequest) (*pb.ActivateResponse, error) {
	if req.Token == "" {
		return nil, status.Error(codes.InvalidArgument, "token required")
	}
	if _, err := h.svc.Activate(ctx, req.Token); err != nil {
		return nil, status.Error(codes.NotFound, "invalid or expired token")
	}
	return &pb.ActivateResponse{}, nil
}

func (h *Handler) GetUser(ctx context.Context, req *pb.GetUserRequest) (*pb.UserResponse, error) {
	user, err := h.svc.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.NotFound, "user not found")
	}
	return userToProto(user), nil
}

func (h *Handler) UpdateProfile(ctx context.Context, req *pb.UpdateProfileRequest) (*pb.UserResponse, error) {
	if err := h.svc.UpdateProfile(ctx, req.UserId, req.DisplayName, req.AvatarUrl, req.Phone, req.Address); err != nil {
		h.log.Errorw("grpc update profile", zap.Error(err))
		return nil, status.Error(codes.Internal, "could not update profile")
	}
	user, err := h.svc.GetByID(ctx, req.UserId)
	if err != nil {
		return nil, status.Error(codes.Internal, "could not fetch updated user")
	}
	return userToProto(user), nil
}

func (h *Handler) Logout(ctx context.Context, req *pb.LogoutRequest) (*pb.LogoutResponse, error) {
	if req.Jti != "" && req.ExpiresAt > 0 {
		ttl := time.Until(time.Unix(req.ExpiresAt, 0))
		if ttl > 0 {
			if err := h.blacklist.Revoke(ctx, req.Jti, ttl); err != nil {
				h.log.Warnw("grpc logout: revoke", "jti", req.Jti, "err", err)
			}
		}
	}
	return &pb.LogoutResponse{}, nil
}

func userToProto(u *domain.User) *pb.UserResponse {
	return &pb.UserResponse{
		Id:          u.ID,
		Email:       u.Email,
		Role:        string(u.Role),
		IsActivated: u.IsActivated,
		CreatedAt:   u.CreatedAt.Unix(),
		DisplayName: u.DisplayName,
		AvatarUrl:   u.AvatarURL,
		Phone:       u.Phone,
		Address:     u.Address,
	}
}
