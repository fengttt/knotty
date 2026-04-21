// Package knot implements pure-Go operations on knots: notation
// conversions, invariant computations, and diagram rendering.
//
// Knot holds the raw strings of a knot_info row. Use Get<Field>() to access
// a value converted to its natural Go type; use Raw to fetch the underlying
// string for any column by name.
package knot

import (
	"fmt"
	"strconv"
	"strings"
)

// ColumnNames returns the knot_info column names in declaration order.
// Callers can use this to iterate all fields in a stable order.
func ColumnNames() []string {
	out := make([]string, len(columns))
	copy(out, columns)
	return out
}


// columns lists every knot_info column name in declaration order.
var columns = []string{
	"name",
	"category",
	"alternating",
	"name_rank",
	"dt_name",
	"dt_rank",
	"dt_notation",
	"classical_conway_name",
	"conway_notation",
	"two_bridge_notation",
	"fibered",
	"gauss_notation",
	"enhanced_gauss_notation",
	"pd_notation",
	"crossing_number",
	"tetrahedral_census_name",
	"unknotting_number",
	"three_genus",
	"crosscap_number",
	"bridge_index",
	"braid_index",
	"braid_length",
	"braid_notation",
	"signature",
	"nakanishi_index",
	"super_bridge_index",
	"thurston_bennequin_number",
	"arc_index",
	"polygon_index",
	"tunnel_number",
	"morse_novikov_number",
	"alexander_polynomial",
	"alexander_polynomial_vector",
	"jones_polynomial",
	"jones_polynomial_vector",
	"conway_polynomial",
	"conway_polynomial_vector",
	"kauffman_polynomial",
	"kauffman_polynomial_vector",
	"a_polynomial",
	"smooth_four_genus",
	"topological_four_genus",
	"smooth_4d_crosscap_number",
	"topological_4d_crosscap_number",
	"smooth_concordance_genus",
	"topological_concordance_genus",
	"smooth_concordance_crosscap_number",
	"topological_concordance_crosscap_number",
	"algebraic_concordance_order",
	"smooth_concordance_order",
	"topological_concordance_order",
	"ribbon",
	"determinant",
	"seifert_matrix",
	"rasmussen_invariant",
	"ozsvath_szabo_tau_invariant",
	"volume",
	"maximum_cusp_volume",
	"longitude_translation",
	"meridian_translation",
	"longitude_length",
	"meridian_length",
	"other_short_geodesics",
	"symmetry_type",
	"full_symmetry_group",
	"chern_simons_invariant",
	"volume_imaginary_part",
	"arf_invariant",
	"turaev_genus",
	"signature_function",
	"monodromy",
	"small_large",
	"positive_braid",
	"positive",
	"strongly_quasipositive",
	"quasipositive",
	"positive_braid_notation",
	"positive_pd_notation",
	"strongly_quasipositive_braid_notation",
	"quasipositive_braid_notation",
	"fd_clasp_number",
	"width",
	"torsion_numbers",
	"td_clasp_number",
	"l_space",
	"nu",
	"epsilon",
	"quasi_alternating",
	"almost_alternating",
	"adequate",
	"montesinos_notation",
	"boundary_slopes",
	"pretzel_notation",
	"double_slice_genus",
	"unknotting_number_algebraic",
	"khovanov_unreduced_integral_polynomial",
	"khovanov_unreduced_integral_vector",
	"khovanov_reduced_integral_polynomial",
	"khovanov_reduced_integral_vector",
	"khovanov_reduced_rational_polynomial",
	"khovanov_reduced_rational_vector",
	"khovanov_reduced_mod2_polynomial",
	"khovanov_reduced_mod2_vector",
	"khovanov_odd_integral_polynomial",
	"khovanov_odd_integral_vector",
	"khovanov_odd_mod2_polynomial",
	"khovanov_odd_mod2_vector",
	"khovanov_odd_rational_polynomial",
	"khovanov_odd_rational_vector",
	"hfk_polynomial",
	"hfk_polynomial_vector",
	"mosaic_tile_number",
	"ropelength",
	"homfly_polynomial",
	"homfly_polynomial_vector",
	"grid_notation",
	"almost_strongly_qp",
	"almost_strongly_qp_braid",
	"ribbon_number",
	"geometric_type",
	"cosmetic_crossing",
	"q_polynomial",
}

// Knot is a knot_info row. All fields hold raw spreadsheet strings.
type Knot struct {
	name                                 string
	category                             string
	alternating                          string
	nameRank                             string
	dtName                               string
	dtRank                               string
	dtNotation                           string
	classicalConwayName                  string
	conwayNotation                       string
	twoBridgeNotation                    string
	fibered                              string
	gaussNotation                        string
	enhancedGaussNotation                string
	pdNotation                           string
	crossingNumber                       string
	tetrahedralCensusName                string
	unknottingNumber                     string
	threeGenus                           string
	crosscapNumber                       string
	bridgeIndex                          string
	braidIndex                           string
	braidLength                          string
	braidNotation                        string
	signature                            string
	nakanishiIndex                       string
	superBridgeIndex                     string
	thurstonBennequinNumber              string
	arcIndex                             string
	polygonIndex                         string
	tunnelNumber                         string
	morseNovikovNumber                   string
	alexanderPolynomial                  string
	alexanderPolynomialVector            string
	jonesPolynomial                      string
	jonesPolynomialVector                string
	conwayPolynomial                     string
	conwayPolynomialVector               string
	kauffmanPolynomial                   string
	kauffmanPolynomialVector             string
	aPolynomial                          string
	smoothFourGenus                      string
	topologicalFourGenus                 string
	smooth4dCrosscapNumber               string
	topological4dCrosscapNumber          string
	smoothConcordanceGenus               string
	topologicalConcordanceGenus          string
	smoothConcordanceCrosscapNumber      string
	topologicalConcordanceCrosscapNumber string
	algebraicConcordanceOrder            string
	smoothConcordanceOrder               string
	topologicalConcordanceOrder          string
	ribbon                               string
	determinant                          string
	seifertMatrix                        string
	rasmussenInvariant                   string
	ozsvathSzaboTauInvariant             string
	volume                               string
	maximumCuspVolume                    string
	longitudeTranslation                 string
	meridianTranslation                  string
	longitudeLength                      string
	meridianLength                       string
	otherShortGeodesics                  string
	symmetryType                         string
	fullSymmetryGroup                    string
	chernSimonsInvariant                 string
	volumeImaginaryPart                  string
	arfInvariant                         string
	turaevGenus                          string
	signatureFunction                    string
	monodromy                            string
	smallLarge                           string
	positiveBraid                        string
	positive                             string
	stronglyQuasipositive                string
	quasipositive                        string
	positiveBraidNotation                string
	positivePdNotation                   string
	stronglyQuasipositiveBraidNotation   string
	quasipositiveBraidNotation           string
	fdClaspNumber                        string
	width                                string
	torsionNumbers                       string
	tdClaspNumber                        string
	lSpace                               string
	nu                                   string
	epsilon                              string
	quasiAlternating                     string
	almostAlternating                    string
	adequate                             string
	montesinosNotation                   string
	boundarySlopes                       string
	pretzelNotation                      string
	doubleSliceGenus                     string
	unknottingNumberAlgebraic            string
	khovanovUnreducedIntegralPolynomial  string
	khovanovUnreducedIntegralVector      string
	khovanovReducedIntegralPolynomial    string
	khovanovReducedIntegralVector        string
	khovanovReducedRationalPolynomial    string
	khovanovReducedRationalVector        string
	khovanovReducedMod2Polynomial        string
	khovanovReducedMod2Vector            string
	khovanovOddIntegralPolynomial        string
	khovanovOddIntegralVector            string
	khovanovOddMod2Polynomial            string
	khovanovOddMod2Vector                string
	khovanovOddRationalPolynomial        string
	khovanovOddRationalVector            string
	hfkPolynomial                        string
	hfkPolynomialVector                  string
	mosaicTileNumber                     string
	ropelength                           string
	homflyPolynomial                     string
	homflyPolynomialVector               string
	gridNotation                         string
	almostStronglyQp                     string
	almostStronglyQpBraid                string
	ribbonNumber                         string
	geometricType                        string
	cosmeticCrossing                     string
	qPolynomial                          string
}

// NewFromRow constructs a Knot from a knot_info row. cols is the column
// name list; vals is the values in the same order. Any unrecognized column
// name is ignored; any unset field stays empty. Returns an error if cols
// and vals have different lengths.
func NewFromRow(cols []string, vals []string) (*Knot, error) {
	if len(cols) != len(vals) {
		return nil, fmt.Errorf("NewFromRow: len(cols)=%d != len(vals)=%d", len(cols), len(vals))
	}
	k := &Knot{}
	for i, c := range cols {
		k.setRaw(c, vals[i])
	}
	return k, nil
}

func (k *Knot) setRaw(col, v string) {
	switch col {
	case "name":
		k.name = v
	case "category":
		k.category = v
	case "alternating":
		k.alternating = v
	case "name_rank":
		k.nameRank = v
	case "dt_name":
		k.dtName = v
	case "dt_rank":
		k.dtRank = v
	case "dt_notation":
		k.dtNotation = v
	case "classical_conway_name":
		k.classicalConwayName = v
	case "conway_notation":
		k.conwayNotation = v
	case "two_bridge_notation":
		k.twoBridgeNotation = v
	case "fibered":
		k.fibered = v
	case "gauss_notation":
		k.gaussNotation = v
	case "enhanced_gauss_notation":
		k.enhancedGaussNotation = v
	case "pd_notation":
		k.pdNotation = v
	case "crossing_number":
		k.crossingNumber = v
	case "tetrahedral_census_name":
		k.tetrahedralCensusName = v
	case "unknotting_number":
		k.unknottingNumber = v
	case "three_genus":
		k.threeGenus = v
	case "crosscap_number":
		k.crosscapNumber = v
	case "bridge_index":
		k.bridgeIndex = v
	case "braid_index":
		k.braidIndex = v
	case "braid_length":
		k.braidLength = v
	case "braid_notation":
		k.braidNotation = v
	case "signature":
		k.signature = v
	case "nakanishi_index":
		k.nakanishiIndex = v
	case "super_bridge_index":
		k.superBridgeIndex = v
	case "thurston_bennequin_number":
		k.thurstonBennequinNumber = v
	case "arc_index":
		k.arcIndex = v
	case "polygon_index":
		k.polygonIndex = v
	case "tunnel_number":
		k.tunnelNumber = v
	case "morse_novikov_number":
		k.morseNovikovNumber = v
	case "alexander_polynomial":
		k.alexanderPolynomial = v
	case "alexander_polynomial_vector":
		k.alexanderPolynomialVector = v
	case "jones_polynomial":
		k.jonesPolynomial = v
	case "jones_polynomial_vector":
		k.jonesPolynomialVector = v
	case "conway_polynomial":
		k.conwayPolynomial = v
	case "conway_polynomial_vector":
		k.conwayPolynomialVector = v
	case "kauffman_polynomial":
		k.kauffmanPolynomial = v
	case "kauffman_polynomial_vector":
		k.kauffmanPolynomialVector = v
	case "a_polynomial":
		k.aPolynomial = v
	case "smooth_four_genus":
		k.smoothFourGenus = v
	case "topological_four_genus":
		k.topologicalFourGenus = v
	case "smooth_4d_crosscap_number":
		k.smooth4dCrosscapNumber = v
	case "topological_4d_crosscap_number":
		k.topological4dCrosscapNumber = v
	case "smooth_concordance_genus":
		k.smoothConcordanceGenus = v
	case "topological_concordance_genus":
		k.topologicalConcordanceGenus = v
	case "smooth_concordance_crosscap_number":
		k.smoothConcordanceCrosscapNumber = v
	case "topological_concordance_crosscap_number":
		k.topologicalConcordanceCrosscapNumber = v
	case "algebraic_concordance_order":
		k.algebraicConcordanceOrder = v
	case "smooth_concordance_order":
		k.smoothConcordanceOrder = v
	case "topological_concordance_order":
		k.topologicalConcordanceOrder = v
	case "ribbon":
		k.ribbon = v
	case "determinant":
		k.determinant = v
	case "seifert_matrix":
		k.seifertMatrix = v
	case "rasmussen_invariant":
		k.rasmussenInvariant = v
	case "ozsvath_szabo_tau_invariant":
		k.ozsvathSzaboTauInvariant = v
	case "volume":
		k.volume = v
	case "maximum_cusp_volume":
		k.maximumCuspVolume = v
	case "longitude_translation":
		k.longitudeTranslation = v
	case "meridian_translation":
		k.meridianTranslation = v
	case "longitude_length":
		k.longitudeLength = v
	case "meridian_length":
		k.meridianLength = v
	case "other_short_geodesics":
		k.otherShortGeodesics = v
	case "symmetry_type":
		k.symmetryType = v
	case "full_symmetry_group":
		k.fullSymmetryGroup = v
	case "chern_simons_invariant":
		k.chernSimonsInvariant = v
	case "volume_imaginary_part":
		k.volumeImaginaryPart = v
	case "arf_invariant":
		k.arfInvariant = v
	case "turaev_genus":
		k.turaevGenus = v
	case "signature_function":
		k.signatureFunction = v
	case "monodromy":
		k.monodromy = v
	case "small_large":
		k.smallLarge = v
	case "positive_braid":
		k.positiveBraid = v
	case "positive":
		k.positive = v
	case "strongly_quasipositive":
		k.stronglyQuasipositive = v
	case "quasipositive":
		k.quasipositive = v
	case "positive_braid_notation":
		k.positiveBraidNotation = v
	case "positive_pd_notation":
		k.positivePdNotation = v
	case "strongly_quasipositive_braid_notation":
		k.stronglyQuasipositiveBraidNotation = v
	case "quasipositive_braid_notation":
		k.quasipositiveBraidNotation = v
	case "fd_clasp_number":
		k.fdClaspNumber = v
	case "width":
		k.width = v
	case "torsion_numbers":
		k.torsionNumbers = v
	case "td_clasp_number":
		k.tdClaspNumber = v
	case "l_space":
		k.lSpace = v
	case "nu":
		k.nu = v
	case "epsilon":
		k.epsilon = v
	case "quasi_alternating":
		k.quasiAlternating = v
	case "almost_alternating":
		k.almostAlternating = v
	case "adequate":
		k.adequate = v
	case "montesinos_notation":
		k.montesinosNotation = v
	case "boundary_slopes":
		k.boundarySlopes = v
	case "pretzel_notation":
		k.pretzelNotation = v
	case "double_slice_genus":
		k.doubleSliceGenus = v
	case "unknotting_number_algebraic":
		k.unknottingNumberAlgebraic = v
	case "khovanov_unreduced_integral_polynomial":
		k.khovanovUnreducedIntegralPolynomial = v
	case "khovanov_unreduced_integral_vector":
		k.khovanovUnreducedIntegralVector = v
	case "khovanov_reduced_integral_polynomial":
		k.khovanovReducedIntegralPolynomial = v
	case "khovanov_reduced_integral_vector":
		k.khovanovReducedIntegralVector = v
	case "khovanov_reduced_rational_polynomial":
		k.khovanovReducedRationalPolynomial = v
	case "khovanov_reduced_rational_vector":
		k.khovanovReducedRationalVector = v
	case "khovanov_reduced_mod2_polynomial":
		k.khovanovReducedMod2Polynomial = v
	case "khovanov_reduced_mod2_vector":
		k.khovanovReducedMod2Vector = v
	case "khovanov_odd_integral_polynomial":
		k.khovanovOddIntegralPolynomial = v
	case "khovanov_odd_integral_vector":
		k.khovanovOddIntegralVector = v
	case "khovanov_odd_mod2_polynomial":
		k.khovanovOddMod2Polynomial = v
	case "khovanov_odd_mod2_vector":
		k.khovanovOddMod2Vector = v
	case "khovanov_odd_rational_polynomial":
		k.khovanovOddRationalPolynomial = v
	case "khovanov_odd_rational_vector":
		k.khovanovOddRationalVector = v
	case "hfk_polynomial":
		k.hfkPolynomial = v
	case "hfk_polynomial_vector":
		k.hfkPolynomialVector = v
	case "mosaic_tile_number":
		k.mosaicTileNumber = v
	case "ropelength":
		k.ropelength = v
	case "homfly_polynomial":
		k.homflyPolynomial = v
	case "homfly_polynomial_vector":
		k.homflyPolynomialVector = v
	case "grid_notation":
		k.gridNotation = v
	case "almost_strongly_qp":
		k.almostStronglyQp = v
	case "almost_strongly_qp_braid":
		k.almostStronglyQpBraid = v
	case "ribbon_number":
		k.ribbonNumber = v
	case "geometric_type":
		k.geometricType = v
	case "cosmetic_crossing":
		k.cosmeticCrossing = v
	case "q_polynomial":
		k.qPolynomial = v
	}
}

// Raw returns the raw string for the given knot_info column name,
// or "" if the column is unknown.
func (k *Knot) Raw(col string) string {
	switch col {
	case "name":
		return k.name
	case "category":
		return k.category
	case "alternating":
		return k.alternating
	case "name_rank":
		return k.nameRank
	case "dt_name":
		return k.dtName
	case "dt_rank":
		return k.dtRank
	case "dt_notation":
		return k.dtNotation
	case "classical_conway_name":
		return k.classicalConwayName
	case "conway_notation":
		return k.conwayNotation
	case "two_bridge_notation":
		return k.twoBridgeNotation
	case "fibered":
		return k.fibered
	case "gauss_notation":
		return k.gaussNotation
	case "enhanced_gauss_notation":
		return k.enhancedGaussNotation
	case "pd_notation":
		return k.pdNotation
	case "crossing_number":
		return k.crossingNumber
	case "tetrahedral_census_name":
		return k.tetrahedralCensusName
	case "unknotting_number":
		return k.unknottingNumber
	case "three_genus":
		return k.threeGenus
	case "crosscap_number":
		return k.crosscapNumber
	case "bridge_index":
		return k.bridgeIndex
	case "braid_index":
		return k.braidIndex
	case "braid_length":
		return k.braidLength
	case "braid_notation":
		return k.braidNotation
	case "signature":
		return k.signature
	case "nakanishi_index":
		return k.nakanishiIndex
	case "super_bridge_index":
		return k.superBridgeIndex
	case "thurston_bennequin_number":
		return k.thurstonBennequinNumber
	case "arc_index":
		return k.arcIndex
	case "polygon_index":
		return k.polygonIndex
	case "tunnel_number":
		return k.tunnelNumber
	case "morse_novikov_number":
		return k.morseNovikovNumber
	case "alexander_polynomial":
		return k.alexanderPolynomial
	case "alexander_polynomial_vector":
		return k.alexanderPolynomialVector
	case "jones_polynomial":
		return k.jonesPolynomial
	case "jones_polynomial_vector":
		return k.jonesPolynomialVector
	case "conway_polynomial":
		return k.conwayPolynomial
	case "conway_polynomial_vector":
		return k.conwayPolynomialVector
	case "kauffman_polynomial":
		return k.kauffmanPolynomial
	case "kauffman_polynomial_vector":
		return k.kauffmanPolynomialVector
	case "a_polynomial":
		return k.aPolynomial
	case "smooth_four_genus":
		return k.smoothFourGenus
	case "topological_four_genus":
		return k.topologicalFourGenus
	case "smooth_4d_crosscap_number":
		return k.smooth4dCrosscapNumber
	case "topological_4d_crosscap_number":
		return k.topological4dCrosscapNumber
	case "smooth_concordance_genus":
		return k.smoothConcordanceGenus
	case "topological_concordance_genus":
		return k.topologicalConcordanceGenus
	case "smooth_concordance_crosscap_number":
		return k.smoothConcordanceCrosscapNumber
	case "topological_concordance_crosscap_number":
		return k.topologicalConcordanceCrosscapNumber
	case "algebraic_concordance_order":
		return k.algebraicConcordanceOrder
	case "smooth_concordance_order":
		return k.smoothConcordanceOrder
	case "topological_concordance_order":
		return k.topologicalConcordanceOrder
	case "ribbon":
		return k.ribbon
	case "determinant":
		return k.determinant
	case "seifert_matrix":
		return k.seifertMatrix
	case "rasmussen_invariant":
		return k.rasmussenInvariant
	case "ozsvath_szabo_tau_invariant":
		return k.ozsvathSzaboTauInvariant
	case "volume":
		return k.volume
	case "maximum_cusp_volume":
		return k.maximumCuspVolume
	case "longitude_translation":
		return k.longitudeTranslation
	case "meridian_translation":
		return k.meridianTranslation
	case "longitude_length":
		return k.longitudeLength
	case "meridian_length":
		return k.meridianLength
	case "other_short_geodesics":
		return k.otherShortGeodesics
	case "symmetry_type":
		return k.symmetryType
	case "full_symmetry_group":
		return k.fullSymmetryGroup
	case "chern_simons_invariant":
		return k.chernSimonsInvariant
	case "volume_imaginary_part":
		return k.volumeImaginaryPart
	case "arf_invariant":
		return k.arfInvariant
	case "turaev_genus":
		return k.turaevGenus
	case "signature_function":
		return k.signatureFunction
	case "monodromy":
		return k.monodromy
	case "small_large":
		return k.smallLarge
	case "positive_braid":
		return k.positiveBraid
	case "positive":
		return k.positive
	case "strongly_quasipositive":
		return k.stronglyQuasipositive
	case "quasipositive":
		return k.quasipositive
	case "positive_braid_notation":
		return k.positiveBraidNotation
	case "positive_pd_notation":
		return k.positivePdNotation
	case "strongly_quasipositive_braid_notation":
		return k.stronglyQuasipositiveBraidNotation
	case "quasipositive_braid_notation":
		return k.quasipositiveBraidNotation
	case "fd_clasp_number":
		return k.fdClaspNumber
	case "width":
		return k.width
	case "torsion_numbers":
		return k.torsionNumbers
	case "td_clasp_number":
		return k.tdClaspNumber
	case "l_space":
		return k.lSpace
	case "nu":
		return k.nu
	case "epsilon":
		return k.epsilon
	case "quasi_alternating":
		return k.quasiAlternating
	case "almost_alternating":
		return k.almostAlternating
	case "adequate":
		return k.adequate
	case "montesinos_notation":
		return k.montesinosNotation
	case "boundary_slopes":
		return k.boundarySlopes
	case "pretzel_notation":
		return k.pretzelNotation
	case "double_slice_genus":
		return k.doubleSliceGenus
	case "unknotting_number_algebraic":
		return k.unknottingNumberAlgebraic
	case "khovanov_unreduced_integral_polynomial":
		return k.khovanovUnreducedIntegralPolynomial
	case "khovanov_unreduced_integral_vector":
		return k.khovanovUnreducedIntegralVector
	case "khovanov_reduced_integral_polynomial":
		return k.khovanovReducedIntegralPolynomial
	case "khovanov_reduced_integral_vector":
		return k.khovanovReducedIntegralVector
	case "khovanov_reduced_rational_polynomial":
		return k.khovanovReducedRationalPolynomial
	case "khovanov_reduced_rational_vector":
		return k.khovanovReducedRationalVector
	case "khovanov_reduced_mod2_polynomial":
		return k.khovanovReducedMod2Polynomial
	case "khovanov_reduced_mod2_vector":
		return k.khovanovReducedMod2Vector
	case "khovanov_odd_integral_polynomial":
		return k.khovanovOddIntegralPolynomial
	case "khovanov_odd_integral_vector":
		return k.khovanovOddIntegralVector
	case "khovanov_odd_mod2_polynomial":
		return k.khovanovOddMod2Polynomial
	case "khovanov_odd_mod2_vector":
		return k.khovanovOddMod2Vector
	case "khovanov_odd_rational_polynomial":
		return k.khovanovOddRationalPolynomial
	case "khovanov_odd_rational_vector":
		return k.khovanovOddRationalVector
	case "hfk_polynomial":
		return k.hfkPolynomial
	case "hfk_polynomial_vector":
		return k.hfkPolynomialVector
	case "mosaic_tile_number":
		return k.mosaicTileNumber
	case "ropelength":
		return k.ropelength
	case "homfly_polynomial":
		return k.homflyPolynomial
	case "homfly_polynomial_vector":
		return k.homflyPolynomialVector
	case "grid_notation":
		return k.gridNotation
	case "almost_strongly_qp":
		return k.almostStronglyQp
	case "almost_strongly_qp_braid":
		return k.almostStronglyQpBraid
	case "ribbon_number":
		return k.ribbonNumber
	case "geometric_type":
		return k.geometricType
	case "cosmetic_crossing":
		return k.cosmeticCrossing
	case "q_polynomial":
		return k.qPolynomial
	}
	return ""
}

// ----- integer getters -----

func (k *Knot) GetNameRank() int { return parseIntLenient(k.nameRank) }
func (k *Knot) GetDtRank() int { return parseIntLenient(k.dtRank) }
func (k *Knot) GetCrossingNumber() int { return parseIntLenient(k.crossingNumber) }
func (k *Knot) GetUnknottingNumber() int { return parseIntLenient(k.unknottingNumber) }
func (k *Knot) GetThreeGenus() int { return parseIntLenient(k.threeGenus) }
func (k *Knot) GetCrosscapNumber() int { return parseIntLenient(k.crosscapNumber) }
func (k *Knot) GetBridgeIndex() int { return parseIntLenient(k.bridgeIndex) }
func (k *Knot) GetBraidIndex() int { return parseIntLenient(k.braidIndex) }
func (k *Knot) GetBraidLength() int { return parseIntLenient(k.braidLength) }
func (k *Knot) GetSignature() int { return parseIntLenient(k.signature) }
func (k *Knot) GetNakanishiIndex() int { return parseIntLenient(k.nakanishiIndex) }
func (k *Knot) GetSuperBridgeIndex() int { return parseIntLenient(k.superBridgeIndex) }
func (k *Knot) GetThurstonBennequinNumber() int { return parseIntLenient(k.thurstonBennequinNumber) }
func (k *Knot) GetArcIndex() int { return parseIntLenient(k.arcIndex) }
func (k *Knot) GetPolygonIndex() int { return parseIntLenient(k.polygonIndex) }
func (k *Knot) GetTunnelNumber() int { return parseIntLenient(k.tunnelNumber) }
func (k *Knot) GetMorseNovikovNumber() int { return parseIntLenient(k.morseNovikovNumber) }
func (k *Knot) GetSmoothFourGenus() int { return parseIntLenient(k.smoothFourGenus) }
func (k *Knot) GetTopologicalFourGenus() int { return parseIntLenient(k.topologicalFourGenus) }
func (k *Knot) GetSmooth4dCrosscapNumber() int { return parseIntLenient(k.smooth4dCrosscapNumber) }
func (k *Knot) GetTopological4dCrosscapNumber() int { return parseIntLenient(k.topological4dCrosscapNumber) }
func (k *Knot) GetSmoothConcordanceGenus() int { return parseIntLenient(k.smoothConcordanceGenus) }
func (k *Knot) GetTopologicalConcordanceGenus() int { return parseIntLenient(k.topologicalConcordanceGenus) }
func (k *Knot) GetSmoothConcordanceCrosscapNumber() int { return parseIntLenient(k.smoothConcordanceCrosscapNumber) }
func (k *Knot) GetTopologicalConcordanceCrosscapNumber() int { return parseIntLenient(k.topologicalConcordanceCrosscapNumber) }
func (k *Knot) GetAlgebraicConcordanceOrder() int { return parseIntLenient(k.algebraicConcordanceOrder) }
func (k *Knot) GetSmoothConcordanceOrder() int { return parseIntLenient(k.smoothConcordanceOrder) }
func (k *Knot) GetTopologicalConcordanceOrder() int { return parseIntLenient(k.topologicalConcordanceOrder) }
func (k *Knot) GetDeterminant() int { return parseIntLenient(k.determinant) }
func (k *Knot) GetRasmussenInvariant() int { return parseIntLenient(k.rasmussenInvariant) }
func (k *Knot) GetOzsvathSzaboTauInvariant() int { return parseIntLenient(k.ozsvathSzaboTauInvariant) }
func (k *Knot) GetArfInvariant() int { return parseIntLenient(k.arfInvariant) }
func (k *Knot) GetTuraevGenus() int { return parseIntLenient(k.turaevGenus) }
func (k *Knot) GetFdClaspNumber() int { return parseIntLenient(k.fdClaspNumber) }
func (k *Knot) GetWidth() int { return parseIntLenient(k.width) }
func (k *Knot) GetTdClaspNumber() int { return parseIntLenient(k.tdClaspNumber) }
func (k *Knot) GetEpsilon() int { return parseIntLenient(k.epsilon) }
func (k *Knot) GetDoubleSliceGenus() int { return parseIntLenient(k.doubleSliceGenus) }
func (k *Knot) GetUnknottingNumberAlgebraic() int { return parseIntLenient(k.unknottingNumberAlgebraic) }
func (k *Knot) GetMosaicTileNumber() int { return parseIntLenient(k.mosaicTileNumber) }
func (k *Knot) GetRibbonNumber() int { return parseIntLenient(k.ribbonNumber) }

// ----- float64 getters -----

func (k *Knot) GetVolume() float64 { return parseFloatLenient(k.volume) }
func (k *Knot) GetMaximumCuspVolume() float64 { return parseFloatLenient(k.maximumCuspVolume) }
func (k *Knot) GetChernSimonsInvariant() float64 { return parseFloatLenient(k.chernSimonsInvariant) }
func (k *Knot) GetVolumeImaginaryPart() float64 { return parseFloatLenient(k.volumeImaginaryPart) }
func (k *Knot) GetRopelength() float64 { return parseFloatLenient(k.ropelength) }

// ----- []int8 notation getters -----

func (k *Knot) GetDtNotation() []int8 { return parseInt8List(k.dtNotation) }
func (k *Knot) GetGaussNotation() []int8 { return parseInt8List(k.gaussNotation) }
func (k *Knot) GetEnhancedGaussNotation() []int8 { return parseInt8List(k.enhancedGaussNotation) }
func (k *Knot) GetBraidNotation() []int8 { return parseInt8List(k.braidNotation) }
func (k *Knot) GetPositiveBraidNotation() []int8 { return parseInt8List(k.positiveBraidNotation) }
func (k *Knot) GetStronglyQuasipositiveBraidNotation() []int8 { return parseInt8List(k.stronglyQuasipositiveBraidNotation) }
func (k *Knot) GetQuasipositiveBraidNotation() []int8 { return parseInt8List(k.quasipositiveBraidNotation) }
func (k *Knot) GetAlmostStronglyQpBraid() []int8 { return parseInt8List(k.almostStronglyQpBraid) }

// ----- [][4]int8 notation getters -----

func (k *Knot) GetPdNotation() [][4]int8 { return parseInt8Tuples4(k.pdNotation) }
func (k *Knot) GetPositivePdNotation() [][4]int8 { return parseInt8Tuples4(k.positivePdNotation) }

// ----- string getters (polynomials, names, Y/N flags, complex-valued text) -----

func (k *Knot) GetName() string { return k.name }
func (k *Knot) GetCategory() string { return k.category }
func (k *Knot) GetAlternating() string { return k.alternating }
func (k *Knot) GetDtName() string { return k.dtName }
func (k *Knot) GetClassicalConwayName() string { return k.classicalConwayName }
func (k *Knot) GetConwayNotation() string { return k.conwayNotation }
func (k *Knot) GetTwoBridgeNotation() string { return k.twoBridgeNotation }
func (k *Knot) GetFibered() string { return k.fibered }
func (k *Knot) GetTetrahedralCensusName() string { return k.tetrahedralCensusName }
func (k *Knot) GetAlexanderPolynomial() string { return k.alexanderPolynomial }
func (k *Knot) GetAlexanderPolynomialVector() string { return k.alexanderPolynomialVector }
func (k *Knot) GetJonesPolynomial() string { return k.jonesPolynomial }
func (k *Knot) GetJonesPolynomialVector() string { return k.jonesPolynomialVector }
func (k *Knot) GetConwayPolynomial() string { return k.conwayPolynomial }
func (k *Knot) GetConwayPolynomialVector() string { return k.conwayPolynomialVector }
func (k *Knot) GetKauffmanPolynomial() string { return k.kauffmanPolynomial }
func (k *Knot) GetKauffmanPolynomialVector() string { return k.kauffmanPolynomialVector }
func (k *Knot) GetAPolynomial() string { return k.aPolynomial }
func (k *Knot) GetRibbon() string { return k.ribbon }
func (k *Knot) GetSeifertMatrix() string { return k.seifertMatrix }
func (k *Knot) GetLongitudeTranslation() string { return k.longitudeTranslation }
func (k *Knot) GetMeridianTranslation() string { return k.meridianTranslation }
func (k *Knot) GetLongitudeLength() string { return k.longitudeLength }
func (k *Knot) GetMeridianLength() string { return k.meridianLength }
func (k *Knot) GetOtherShortGeodesics() string { return k.otherShortGeodesics }
func (k *Knot) GetSymmetryType() string { return k.symmetryType }
func (k *Knot) GetFullSymmetryGroup() string { return k.fullSymmetryGroup }
func (k *Knot) GetSignatureFunction() string { return k.signatureFunction }
func (k *Knot) GetMonodromy() string { return k.monodromy }
func (k *Knot) GetSmallLarge() string { return k.smallLarge }
func (k *Knot) GetPositiveBraid() string { return k.positiveBraid }
func (k *Knot) GetPositive() string { return k.positive }
func (k *Knot) GetStronglyQuasipositive() string { return k.stronglyQuasipositive }
func (k *Knot) GetQuasipositive() string { return k.quasipositive }
func (k *Knot) GetTorsionNumbers() string { return k.torsionNumbers }
func (k *Knot) GetLSpace() string { return k.lSpace }
func (k *Knot) GetNu() string { return k.nu }
func (k *Knot) GetQuasiAlternating() string { return k.quasiAlternating }
func (k *Knot) GetAlmostAlternating() string { return k.almostAlternating }
func (k *Knot) GetAdequate() string { return k.adequate }
func (k *Knot) GetMontesinosNotation() string { return k.montesinosNotation }
func (k *Knot) GetBoundarySlopes() string { return k.boundarySlopes }
func (k *Knot) GetPretzelNotation() string { return k.pretzelNotation }
func (k *Knot) GetKhovanovUnreducedIntegralPolynomial() string { return k.khovanovUnreducedIntegralPolynomial }
func (k *Knot) GetKhovanovUnreducedIntegralVector() string { return k.khovanovUnreducedIntegralVector }
func (k *Knot) GetKhovanovReducedIntegralPolynomial() string { return k.khovanovReducedIntegralPolynomial }
func (k *Knot) GetKhovanovReducedIntegralVector() string { return k.khovanovReducedIntegralVector }
func (k *Knot) GetKhovanovReducedRationalPolynomial() string { return k.khovanovReducedRationalPolynomial }
func (k *Knot) GetKhovanovReducedRationalVector() string { return k.khovanovReducedRationalVector }
func (k *Knot) GetKhovanovReducedMod2Polynomial() string { return k.khovanovReducedMod2Polynomial }
func (k *Knot) GetKhovanovReducedMod2Vector() string { return k.khovanovReducedMod2Vector }
func (k *Knot) GetKhovanovOddIntegralPolynomial() string { return k.khovanovOddIntegralPolynomial }
func (k *Knot) GetKhovanovOddIntegralVector() string { return k.khovanovOddIntegralVector }
func (k *Knot) GetKhovanovOddMod2Polynomial() string { return k.khovanovOddMod2Polynomial }
func (k *Knot) GetKhovanovOddMod2Vector() string { return k.khovanovOddMod2Vector }
func (k *Knot) GetKhovanovOddRationalPolynomial() string { return k.khovanovOddRationalPolynomial }
func (k *Knot) GetKhovanovOddRationalVector() string { return k.khovanovOddRationalVector }
func (k *Knot) GetHfkPolynomial() string { return k.hfkPolynomial }
func (k *Knot) GetHfkPolynomialVector() string { return k.hfkPolynomialVector }
func (k *Knot) GetHomflyPolynomial() string { return k.homflyPolynomial }
func (k *Knot) GetHomflyPolynomialVector() string { return k.homflyPolynomialVector }
func (k *Knot) GetGridNotation() string { return k.gridNotation }
func (k *Knot) GetAlmostStronglyQp() string { return k.almostStronglyQp }
func (k *Knot) GetGeometricType() string { return k.geometricType }
func (k *Knot) GetCosmeticCrossing() string { return k.cosmeticCrossing }
func (k *Knot) GetQPolynomial() string { return k.qPolynomial }


// parseIntLenient parses a decimal integer, returning 0 for empty or
// unparseable input. KnotInfo sometimes stores ranges (e.g. "[1,2]") for
// uncertain values; those parse as 0 here — fall back to Raw() if the
// exact string matters.
func parseIntLenient(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}

// parseFloatLenient parses a float64, returning 0 for empty or
// unparseable input.
func parseFloatLenient(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

// parseInt8List parses a flat list of signed integers formatted as
// "[a, b, c, ...]" into a []int8. Returns nil for empty input.
func parseInt8List(s string) []int8 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]int8, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		n, err := strconv.ParseInt(p, 10, 8)
		if err != nil {
			return nil
		}
		out = append(out, int8(n))
	}
	return out
}

// parseInt8Tuples4 parses a list of 4-tuples formatted as
// "[[a,b,c,d],[e,f,g,h],...]" into a [][4]int8. Returns nil for empty input
// or when any tuple does not have length 4.
func parseInt8Tuples4(s string) [][4]int8 {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	s = strings.TrimPrefix(s, "[")
	s = strings.TrimSuffix(s, "]")
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	var out [][4]int8
	depth := 0
	start := -1
	for i, r := range s {
		switch r {
		case '[':
			if depth == 0 {
				start = i + 1
			}
			depth++
		case ']':
			depth--
			if depth == 0 && start >= 0 {
				tuple := s[start:i]
				parts := strings.Split(tuple, ",")
				if len(parts) != 4 {
					return nil
				}
				var t [4]int8
				for j, p := range parts {
					p = strings.TrimSpace(p)
					n, err := strconv.ParseInt(p, 10, 8)
					if err != nil {
						return nil
					}
					t[j] = int8(n)
				}
				out = append(out, t)
				start = -1
			}
		}
	}
	if depth != 0 {
		return nil
	}
	return out
}

