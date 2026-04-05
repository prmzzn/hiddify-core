package profile

import (
	context "context"
	"fmt"
	"os"
	"time"

	"github.com/hiddify/hiddify-core/v2/config"
	"github.com/hiddify/hiddify-core/v2/db"
	"github.com/hiddify/hiddify-core/v2/hcommon"
	"github.com/hiddify/hiddify-core/v2/hcommon/request"
)

// RunInstanceFunc is a function type that starts a temporary hcore instance for
// network operations (e.g., downloading profiles through a proxy). It is
// injected at startup to break the import cycle between the profile and hcore
// packages.
//
// The returned object must expose a ListenPort field (uint16) and a
// PingCloudflare() method; we model this with the RunInstanceResult interface.
type RunInstanceResult interface {
	PingCloudflare() (time.Duration, error)
	GetListenPort() uint16
}

// ParseContentFunc is a function type that validates raw sing-box / Hiddify
// profile content. It returns an error if the content is invalid.
type ParseContentFunc func(ctx context.Context, content string) error

var (
	// runInstanceFn is the injected RunInstance implementation.
	runInstanceFn func(ctx context.Context, hiddifySettings *config.HiddifyOptions) (RunInstanceResult, error)

	// parseContentFn is the injected Parse implementation.
	parseContentFn ParseContentFunc
)

// SetRunInstanceFunc sets the factory function used by the profile package to
// start temporary hcore instances. Must be called before AddByUrl / UpdateContent.
func SetRunInstanceFunc(fn func(ctx context.Context, hiddifySettings *config.HiddifyOptions) (RunInstanceResult, error)) {
	runInstanceFn = fn
}

// SetParseContentFunc sets the function used by the profile package to validate
// profile content. Must be called before AddByContent / UpdateContent.
func SetParseContentFunc(fn ParseContentFunc) {
	parseContentFn = fn
}

const (
	profilesDirName = "data/profiles"
)

type ProfileRepositoryServer struct {
	UnimplementedProfileServiceServer
}

func (s *ProfileRepositoryServer) GetProfile(ctx context.Context, req *ProfileRequest) (*ProfileResponse, error) {
	var profile *ProfileEntity
	var err error

	switch {
	case req.Id != "":
		profile, err = GetById(req.Id)
	case req.Name != "":
		profile, err = GetByName(req.Name)
	case req.Url != "":
		profile, err = GetByUrl(ctx, req.Url)
	default:
		return nil, fmt.Errorf("invalid request: %v", req)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching profile: %v", err)
	}

	return &ProfileResponse{Profile: profile}, nil
}

func (s *ProfileRepositoryServer) AddProfile(ctx context.Context, req *AddProfileRequest) (*ProfileResponse, error) {
	var profile *ProfileEntity
	var err error

	switch {
	case req.Url != "":
		profile, err = AddByUrl(ctx, req.Url, req.Name, req.MarkAsActive)

	case req.Content != "":
		profile, err = AddByContent(ctx, req.Content, req.Name, req.MarkAsActive)
	default:
		return nil, fmt.Errorf("invalid request: %v", req)
	}

	if err != nil {
		return nil, fmt.Errorf("error fetching profile: %v", err)
	}

	return &ProfileResponse{Profile: profile}, nil
}

func (s *ProfileRepositoryServer) DeleteProfile(ctx context.Context, req *ProfileRequest) (*hcommon.Response, error) {
	var err error
	switch {
	case req.Id != "":
		err = DeleteById(req.Id)
	default:
		profile, err1 := s.GetProfile(ctx, req)

		if profile.Profile == nil {
			err = fmt.Errorf("error deleting profile: %v", err1)
		} else {
			err = DeleteById(profile.Profile.Id)
		}
	}

	if err != nil {
		return &hcommon.Response{Message: err.Error(), Code: hcommon.ResponseCode_FAILED}, fmt.Errorf("error deleting profile: %v", err)
	}

	return &hcommon.Response{Code: hcommon.ResponseCode_OK}, nil
}

func (s *ProfileRepositoryServer) SetActiveProfile(ctx context.Context, req *ProfileRequest) (*hcommon.Response, error) {
	var err error
	switch {
	case req.Id != "":

		var profile *ProfileEntity
		profile, err = GetById(req.Id)
		if err == nil {
			err = SetActiveProfile(profile)
		}
	default:

		var profile *ProfileResponse
		profile, err = s.GetProfile(ctx, req)

		if profile.Profile == nil {
			err = fmt.Errorf("error setting profile as active: %v", err)
		} else {
			err = SetActiveProfile(profile.Profile)
		}
	}

	if err != nil {
		return &hcommon.Response{Message: err.Error(), Code: hcommon.ResponseCode_FAILED}, fmt.Errorf("error setting profile as active: %v", err)
	}

	return &hcommon.Response{Code: hcommon.ResponseCode_OK}, nil
}

func (s *ProfileRepositoryServer) GetAllProfiles(ctx context.Context, req *hcommon.Empty) (*MultiProfilesResponse, error) {
	profiles, err := GetAll()
	if err != nil {
		return &MultiProfilesResponse{ResponseCode: hcommon.ResponseCode_FAILED, Message: err.Error()}, fmt.Errorf("error fetching profiles: %v", err)
	}
	return &MultiProfilesResponse{Profiles: profiles}, nil
}

func (s *ProfileRepositoryServer) UpdateProfile(ctx context.Context, req *ProfileEntity) (*ProfileResponse, error) {
	err := UpdateProfile(req)
	if err != nil {
		return &ProfileResponse{ResponseCode: hcommon.ResponseCode_FAILED, Message: err.Error()}, fmt.Errorf("error updating profile: %v", err)
	}
	return &ProfileResponse{ResponseCode: hcommon.ResponseCode_OK, Profile: req}, nil
}

func GetAll() ([]*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	allEntities, err := table.All()
	return allEntities, err
}

func (s *ProfileRepositoryServer) GetActiveProfile(ctx context.Context, req *hcommon.Empty) (*ProfileResponse, error) {
	profile, err := GetActiveProfile()
	if err != nil {
		return &ProfileResponse{ResponseCode: hcommon.ResponseCode_FAILED, Message: err.Error()}, fmt.Errorf("error fetching active profile: %v", err)
	}
	return &ProfileResponse{Profile: profile}, nil
}

func GetActiveProfile() (*ProfileEntity, error) {
	table := db.GetTable[hcommon.AppSettings]()
	active, err := table.Get("active_profile")
	if err != nil {
		return nil, err
	}
	prof, err := GetById(active.Value.(string))
	if err != nil {
		return nil, err
	}
	return prof, nil
}

func SetActiveProfile(entity *ProfileEntity) error {
	table := db.GetTable[hcommon.AppSettings]()
	return table.UpdateInsert(&hcommon.AppSettings{
		Id:    "active_profile",
		Value: entity.Id,
	})
}

func GetById(id string) (*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	entity, err := table.Get(id)
	if err != nil {
		return nil, fmt.Errorf("error fetching profile by ID: %v", err)
	}
	return entity, nil
}

func GetByName(name string) (*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	allEntities, err := table.All()
	for _, entity := range allEntities {
		if entity.Name == name {
			return entity, nil
		}
	}

	return nil, fmt.Errorf("error fetching profile by ID: %v", err)
}

func GetByUrl(ctx context.Context, url string) (*ProfileEntity, error) {
	table := db.GetTable[ProfileEntity]()
	allEntities, err := table.All()
	for _, entity := range allEntities {
		if entity.Url == url {
			return entity, nil
		}
	}

	return nil, fmt.Errorf("error fetching profile by ID: %v", err)
}

func AddByUrl(ctx context.Context, url string, optionalName string, markAsActive bool) (*ProfileEntity, error) {
	existingProfile, _ := GetByUrl(ctx, url)
	if existingProfile != nil {
		// If the profile already exists, update it
		return existingProfile, UpdateSubscription(existingProfile, false)
	}

	profileId := generateUuid()

	// Attempt to download the profile content
	content, err := downloadProfileContent(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("error downloading profile: %v", err)
	}

	err = UpdateContent(ctx, profileId, content.Body)
	if err != nil {
		return nil, fmt.Errorf("error updating profile content: %v", err)
	}

	newProfile := &ProfileEntity{
		Id: profileId,

		LastUpdate: time.Now().UnixMicro(),
		Url:        url,
	}
	newProfile.Parse(content.Header)
	if optionalName != "" {
		newProfile.Name = optionalName
	}
	table := db.GetTable[ProfileEntity]()
	if err := table.UpdateInsert(newProfile); err != nil {
		return nil, fmt.Errorf("error inserting new profile into the database: %v", err)
	}

	if markAsActive {
		SetActiveProfile(newProfile)
	}
	return newProfile, nil
}

// downloadProfileContent handles the download logic
func downloadProfileContent(ctx context.Context, url string) (*request.Response, error) {
	resp, err := request.Send(request.Request{
		Url:       url,
		Method:    request.GET,
		SocksPort: 12334,
		Timeout:   5 * time.Second,
	})
	if resp == nil {
		resp, err = request.Send(request.Request{
			Url:     url,
			Method:  request.GET,
			Timeout: 5 * time.Second,
		})
		if resp == nil {
			if runInstanceFn == nil {
				return nil, fmt.Errorf("%v, no runInstanceFn registered", err)
			}
			instance, err1 := runInstanceFn(ctx, config.DefaultHiddifyOptions())
			if err1 != nil {
				return nil, fmt.Errorf("%v,error running instance: %v", err, err1)
			}
			instance.PingCloudflare()
			resp, err1 = request.Send(request.Request{
				Url:       url,
				Method:    request.GET,
				Timeout:   5 * time.Second,
				SocksPort: instance.GetListenPort(),
			})
			if err1 != nil {
				err = fmt.Errorf("%v, Fragment: %v", err, err1)
			}
		}
	}
	if resp == nil {
		return nil, err
	}
	if resp.StatusCode == 401 {
		return nil, fmt.Errorf("Authentication error: %v", resp.StatusCode)
	}
	contentHeaders := parseHeadersFromContent(resp.Body)
	for k, v := range contentHeaders {
		resp.Header.Set(k, v[0])
	}
	return resp, nil
}

func UpdateContent(ctx context.Context, profileId, content string) error {
	if _, err := os.Stat(profilesDirName); os.IsNotExist(err) {
		err := os.MkdirAll(profilesDirName, 0o755)
		if err != nil {
			return fmt.Errorf("failed to create directory: %v", err)
		}
	}

	if parseContentFn != nil {
		if err := parseContentFn(ctx, content); err != nil {
			return err
		}
	}

	return os.WriteFile(profilesDirName+"/"+profileId+".info", []byte(content), 0o644)
}

func AddByContent(ctx context.Context, content, name string, markAsActive bool) (*ProfileEntity, error) {
	profileId := generateUuid()

	err := UpdateContent(ctx, profileId, content)
	if err != nil {
		return nil, err
	}

	newProfile := &ProfileEntity{
		Id:         profileId,
		Name:       name,
		LastUpdate: time.Now().UnixMilli(),
	}

	table := db.GetTable[ProfileEntity]()
	if err := table.UpdateInsert(newProfile); err != nil {
		return nil, fmt.Errorf("error inserting new profile into the database: %v", err)
	}
	if markAsActive {
		SetActiveProfile(newProfile)
	}
	return newProfile, nil
}

func UpdateSubscription(baseProfile *ProfileEntity, patchBaseProfile bool) error {
	return nil
}

func Patch(profile *ProfileEntity) error {
	// Implement patch logic
	return nil
}

func DeleteById(id string) error {
	table := db.GetTable[ProfileEntity]()
	os.Remove(profilesDirName + "/" + id + ".info")
	return table.Delete(id)
}

func UpdateProfile(profile *ProfileEntity) error {
	table := db.GetTable[ProfileEntity]()
	return table.UpdateInsert(profile)
}
