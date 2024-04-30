package schema

type Audio string

const (
	AudioPortuguese  = "Português"
	AudioPortuguese2 = "Portugues"
	AudioEnglish     = "Inglês"
	AudioEnglish2    = "Ingles"
	AudioSpanish     = "Espanhol"
	AudioFrench      = "Francês"
	AudioFrench2     = "Frances"
	AudioGerman      = "Alemão"
	AudioGerman2     = "Alemao"
	AudioItalian     = "Italiano"
	AudioJapanese    = "Japonês"
	AudioJapanese2   = "Japones"
	AudioKorean      = "Coreano"
	AudioMandarin    = "Mandarim"
	AudioMandarin2   = "Chinês"
	AudioMandarin3   = "Chines"
	AudioRussian     = "Russo"
	AudioSwedish     = "Sueco"
	AudioUkrainian   = "Ucraniano"
	AudioPolish      = "Polaco"
	AudioPolish2     = "Polonês"
	AudioPolish3     = "Polones"
	AudioThai        = "Tailandês"
	AudioThai2       = "Tailandes"
	AudioTurkish     = "Turco"
)

var AudioList = []Audio{
	AudioPortuguese,
	AudioPortuguese2,
	AudioEnglish,
	AudioEnglish2,
	AudioSpanish,
	AudioFrench,
	AudioFrench2,
	AudioGerman,
	AudioGerman2,
	AudioItalian,
	AudioJapanese,
	AudioJapanese2,
	AudioKorean,
	AudioMandarin,
	AudioMandarin2,
	AudioMandarin3,
	AudioRussian,
	AudioSwedish,
	AudioUkrainian,
	AudioPolish,
	AudioPolish2,
	AudioPolish3,
	AudioThai,
	AudioThai2,
	AudioTurkish,
}

func (a Audio) String() string {
	return a.toISO639_2()
}

func GetAudioFromString(s string) *Audio {
	for _, a := range AudioList {
		if string(a) == s {
			return &a
		}
	}
	return nil
}

func (a Audio) toISO639_2() string {
	switch a {
	case AudioPortuguese:
		return "pt-br"
	case AudioPortuguese2:
		return "pt-br"
	case AudioEnglish:
		return "eng"
	case AudioEnglish2:
		return "eng"
	case AudioSpanish:
		return "spa"
	case AudioFrench:
		return "fra"
	case AudioFrench2:
		return "fra"
	case AudioGerman:
		return "deu"
	case AudioGerman2:
		return "deu"
	case AudioItalian:
		return "ita"
	case AudioJapanese:
		return "jpn"
	case AudioJapanese2:
		return "jpn"
	case AudioKorean:
		return "kor"
	case AudioMandarin:
		return "chi"
	case AudioMandarin2:
		return "chi"
	case AudioMandarin3:
		return "chi"
	case AudioRussian:
		return "rus"
	case AudioSwedish:
		return "swe"
	case AudioUkrainian:
		return "ukr"
	case AudioPolish:
		return "pol"
	case AudioPolish2:
		return "pol"
	case AudioPolish3:
		return "pol"
	case AudioThai:
		return "tha"
	case AudioThai2:
		return "tha"
	case AudioTurkish:
		return "tur"
	default:
		return ""
	}
}
