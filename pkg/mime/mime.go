package mime

import "mime"

type Map = map[string]string

func BatchRegister(m Map) (err error) {
	for e, t := range m {
		if err := mime.AddExtensionType(e, t); err != nil {
			return err
		}
	}

	return nil
}
