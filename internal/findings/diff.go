package findings

// Diff compara duas listas de achados pelo Fingerprint: novos estão em curr
// e não em prev; resolvidos estavam em prev e sumiram de curr.
func Diff(prev, curr []Finding) (novos, resolvidos []Finding) {
	before := make(map[string]bool, len(prev))
	for _, f := range prev {
		before[f.Fingerprint] = true
	}
	now := make(map[string]bool, len(curr))
	novos, resolvidos = []Finding{}, []Finding{}
	for _, f := range curr {
		now[f.Fingerprint] = true
		if !before[f.Fingerprint] {
			novos = append(novos, f)
		}
	}
	for _, f := range prev {
		if !now[f.Fingerprint] {
			resolvidos = append(resolvidos, f)
		}
	}
	return novos, resolvidos
}
