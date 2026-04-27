package profile

type Store interface {
	Load() (Config, error)
	Save(Config) error
}

type Service struct {
	store Store
}

func NewService(store Store) Service {
	return Service{store: store}
}

func (s Service) LoadConfig() (Config, error) {
	return s.store.Load()
}

func (s Service) SaveConfig(cfg Config) error {
	return s.store.Save(cfg)
}

func (s Service) Set(kind string, name string, p ExternalProfile) error {
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	cfg.SetExternalProfile(kind, name, p)
	return s.store.Save(cfg)
}

func (s Service) Delete(kind string, name string) error {
	cfg, err := s.store.Load()
	if err != nil {
		return err
	}
	cfg.DeleteExternalProfile(kind, name)
	return s.store.Save(cfg)
}
