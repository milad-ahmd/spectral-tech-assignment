package grpcserver

import (
	"context"
	"errors"
	"time"

	meterusagev1 "github.com/milad/spectral/gen/go/proto/meterusage/v1"
	"github.com/milad/spectral/internal/domain"
	"github.com/milad/spectral/internal/service"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Server struct {
	meterusagev1.UnimplementedMeterUsageServiceServer
	svc *service.MeterUsageService
}

func New(svc *service.MeterUsageService) *Server {
	return &Server{svc: svc}
}

func (s *Server) ListReadings(ctx context.Context, req *meterusagev1.ListReadingsRequest) (*meterusagev1.ListReadingsResponse, error) {
	if req == nil {
		return nil, status.Error(codes.InvalidArgument, "request is required")
	}
	start, end, err := fromProtoRange(req.GetStart(), req.GetEnd())
	if err != nil {
		return nil, status.Error(codes.InvalidArgument, err.Error())
	}

	res, err := s.svc.ListReadingsPage(ctx, start, end, int(req.GetPageSize()), req.GetPageToken())
	if err != nil {
		if errors.Is(err, service.ErrInvalidTimeRange) || errors.Is(err, service.ErrInvalidPagination) {
			return nil, status.Error(codes.InvalidArgument, err.Error())
		}
		return nil, status.Error(codes.Internal, "internal error")
	}

	out := make([]*meterusagev1.Reading, 0, len(res.Readings))
	for _, r := range res.Readings {
		out = append(out, toProtoReading(r))
	}
	return &meterusagev1.ListReadingsResponse{
		Readings:      out,
		NextPageToken: res.NextPageToken,
	}, nil
}

func toProtoReading(r domain.Reading) *meterusagev1.Reading {
	return &meterusagev1.Reading{
		Time:       timestamppb.New(r.Time),
		MeterUsage: r.MeterUsage,
	}
}

func fromProtoRange(start, end *timestamppb.Timestamp) (*time.Time, *time.Time, error) {
	var (
		s *time.Time
		e *time.Time
	)
	if start != nil {
		if err := start.CheckValid(); err != nil {
			return nil, nil, err
		}
		t := start.AsTime().UTC()
		s = &t
	}
	if end != nil {
		if err := end.CheckValid(); err != nil {
			return nil, nil, err
		}
		t := end.AsTime().UTC()
		e = &t
	}
	return s, e, nil
}
