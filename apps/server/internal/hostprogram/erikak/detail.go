package erikak

import (
	"errors"
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

func detailFor(host world.Host, store world.Store) hostcatalog.ErikaKDetail {
	if s, ok := store.(world.HostDetailStore); ok {
		if d, ok := s.HostDetail(host.ID); ok && d.ErikaK != nil {
			return *d.ErikaK
		}
	}
	return hostcatalog.ErikaKDetail{}
}

func ValidateDetail(d hostcatalog.ErikaKDetail) error {
	var errs []error
	check := func(field, text string) {
		if displayCellWidth(strings.ReplaceAll(text, "{handle}", "12345678")) > 80 {
			errs = append(errs, fmt.Errorf("%s: display width exceeds 80 cells", field))
		}
	}
	for i, line := range d.Texts.LoginBanner {
		check(fmt.Sprintf("texts.login_banner[%d]", i), line)
	}
	check("texts.login_greeting", d.Texts.LoginGreeting)
	check("texts.main_menu_title", d.Texts.MainMenuTitle)
	check("texts.goodbye", d.Texts.Goodbye)
	return errors.Join(errs...)
}
