//go:build integration

package repository

import (
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

func (s *AccountRepoSuite) TestList_DefaultSortByNameAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "z-account"})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "a-account"})

	accounts, _, err := s.repo.List(s.ctx, pagination.PaginationParams{Page: 1, PageSize: 10})
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("a-account", accounts[0].Name)
	s.Require().Equal("z-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByPriorityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "low-priority", Priority: 10})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "high-priority", Priority: 90})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "priority",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("high-priority", accounts[0].Name)
	s.Require().Equal("low-priority", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByTypeAsc() {
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "oauth-account", Type: service.AccountTypeOAuth})
	mustCreateAccount(s.T(), s.client, &service.Account{Name: "apikey-account", Type: service.AccountTypeAPIKey})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "type",
		SortOrder: "asc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 2)
	s.Require().Equal("apikey-account", accounts[0].Name)
	s.Require().Equal("oauth-account", accounts[1].Name)
}

func (s *AccountRepoSuite) TestListWithFilters_SortByAvailabilityDesc() {
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "available-account",
		Status:      service.StatusActive,
		Schedulable: true,
	})
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name:             "rate-limited-account",
		Status:           service.StatusActive,
		Schedulable:      true,
		RateLimitResetAt: ptrTime(time.Now().Add(time.Hour)),
	})
	mustCreateAccount(s.T(), s.client, &service.Account{
		Name:        "inactive-account",
		Status:      service.StatusDisabled,
		Schedulable: true,
	})

	accounts, _, err := s.repo.ListWithFilters(s.ctx, pagination.PaginationParams{
		Page:      1,
		PageSize:  10,
		SortBy:    "availability",
		SortOrder: "desc",
	}, "", "", "", "", 0, "")
	s.Require().NoError(err)
	s.Require().Len(accounts, 3)
	s.Require().Equal("available-account", accounts[0].Name)
}
