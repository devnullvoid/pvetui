package ui

// ToFloat converts interface{} to float64
func ToFloat(val interface{}) float64 {
	switch v := val.(type) {
	case float64:
		return v
	case int:
		return float64(v)
	default:
		return 0
	}
}

// // toFloat converts interface{} to float64
// func toFloat(val interface{}) float64 {
// 	switch v := val.(type) {
// 	case float64:
// 		return v
// 	case int:
// 		return float64(v)
// 	default:
// 		return 0
// 	}
// }
