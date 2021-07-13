package service

func (svc *Service) CountUser() (int, error) {
	return svc.dao.CountUser()
}
