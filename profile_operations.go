package main

import "context"

func (m *ProfileService) StartFacebookPageFetch(ctx context.Context, profileID, pageURL string, count int) (FacebookPageFetchStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		return FacebookPageFetchStatus{}, err
	}
	return s.StartFacebookPageFetch(ctx, pageURL, count)
}

func (m *ProfileService) GetFacebookPageFetchStatus(profileID string) (FacebookPageFetchStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		return FacebookPageFetchStatus{}, err
	}
	return s.GetFacebookPageFetchStatus(), nil
}

func (m *ProfileService) CancelFacebookPageFetch(profileID string, runID uint64) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	s.CancelFacebookPageFetch(runID)
	return nil
}

// Account operations always resolve a session before starting work.

func (m *ProfileService) GetConnectivityStatus(profileID string) (ConnectivityStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero ConnectivityStatus
		return zero, err
	}
	return s.GetConnectivityStatus(), nil
}

func (m *ProfileService) GetLoginState(ctx context.Context, profileID string) (string, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero string
		return zero, err
	}
	return s.GetLoginState(ctx)
}

func (m *ProfileService) GetGroupsLoadStatus(profileID string) (GroupsLoadStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero GroupsLoadStatus
		return zero, err
	}
	return s.GetGroupsLoadStatus(), nil
}

func (m *ProfileService) OpenPostComposer(ctx context.Context, profileID string, selectedIDs []string, body string, images []PostUpload) (*PostPreparation, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero *PostPreparation
		return zero, err
	}
	return s.OpenPostComposer(ctx, selectedIDs, body, images)
}

func (m *ProfileService) GetPostPreparationStatus(profileID string) (string, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero string
		return zero, err
	}
	return s.GetPostPreparationStatus(), nil
}

func (m *ProfileService) SearchGroups(ctx context.Context, profileID string, query string) ([]GroupSearchHit, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero []GroupSearchHit
		return zero, err
	}
	return s.SearchGroups(ctx, query)
}

func (m *ProfileService) OpenJoinedGroups(ctx context.Context, profileID string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	return s.OpenJoinedGroups(ctx)
}

func (m *ProfileService) OpenGroupsSortMenu(ctx context.Context, profileID string) (bool, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero bool
		return zero, err
	}
	return s.OpenGroupsSortMenu(ctx)
}

func (m *ProfileService) SelectUnselectedGroupsSortOption(ctx context.Context, profileID string) (bool, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero bool
		return zero, err
	}
	return s.SelectUnselectedGroupsSortOption(ctx)
}

func (m *ProfileService) PublishRichPostWithOptions(ctx context.Context, profileID string, selected []string, body, bodyHTML string, images []PostUpload, options PostingOptions) (*PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero *PostingStatus
		return zero, err
	}
	return s.PublishRichPostWithOptions(ctx, selected, body, bodyHTML, images, options)
}

func (m *ProfileService) GetPostingStatus(profileID string) (PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero PostingStatus
		return zero, err
	}
	return s.GetPostingStatus(), nil
}

func (m *ProfileService) PublishPostWithOptions(ctx context.Context, profileID string, selected []string, body string, images []PostUpload, options PostingOptions) (*PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero *PostingStatus
		return zero, err
	}
	return s.PublishPostWithOptions(ctx, selected, body, images, options)
}

func (m *ProfileService) CopyPostingResponse(profileID string, runID uint64, logID int) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	return s.CopyPostingResponse(runID, logID)
}

func (m *ProfileService) GetPostingLogs(profileID string, page, pageSize int) (PostingLogsPage, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero PostingLogsPage
		return zero, err
	}
	return s.GetPostingLogs(page, pageSize), nil
}

func (m *ProfileService) OpenPostingLogs(profileID string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	return s.OpenPostingLogs()
}

func (m *ProfileService) ScheduleRichPostWithOptions(ctx context.Context, profileID string, selected []string, body, bodyHTML string, images []PostUpload, options PostingOptions, scheduledAt string) (*PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero *PostingStatus
		return zero, err
	}
	return s.ScheduleRichPostWithOptions(ctx, selected, body, bodyHTML, images, options, scheduledAt)
}

func (m *ProfileService) GetSettings(profileID string) (AppSettings, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero AppSettings
		return zero, err
	}
	return s.GetSettings()
}

func (m *ProfileService) SaveSettings(profileID string, settings AppSettings, key string, clearKey bool) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	return s.SaveSettings(settings, key, clearKey)
}

func (m *ProfileService) PublishPost(ctx context.Context, profileID string, selected []string, body string, images []PostUpload) (*PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		var zero *PostingStatus
		return zero, err
	}
	return s.PublishPost(ctx, selected, body, images)
}

func (m *ProfileService) CancelPosting(profileID string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	s.CancelPosting()
	return nil
}

func (m *ProfileService) PausePosting(profileID string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	s.PausePosting()
	return nil
}

func (m *ProfileService) ResumePosting(profileID string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	s.ResumePosting()
	return nil
}

func (m *ProfileService) PublishFacebookPostsWithOptions(ctx context.Context, profileID string, selected []string, posts []FacebookPagePost, options PostingOptions) (*PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		return nil, err
	}
	return s.PublishFacebookPostsWithOptions(ctx, selected, posts, options)
}
func (m *ProfileService) ScheduleFacebookPostsWithOptions(ctx context.Context, profileID string, selected []string, posts []FacebookPagePost, options PostingOptions, scheduledAt string) (*PostingStatus, error) {
	s, err := m.session(profileID)
	if err != nil {
		return nil, err
	}
	return s.ScheduleFacebookPostsWithOptions(ctx, selected, posts, options, scheduledAt)
}

func (m *ProfileService) RefreshConnectivity(profileID string) error {
	s, err := m.session(profileID)
	if err != nil {
		return err
	}
	s.RefreshConnectivity()
	return nil
}
