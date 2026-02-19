package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

func containsString(list []string, value string) bool {
	for _, entry := range list {
		if entry == value {
			return true
		}
	}
	return false
}

func clamp01(value float64) float64 {
	if value < 0 {
		return 0
	}
	if value > 1 {
		return 1
	}
	return value
}

func haversineMeters(lat1, lng1, lat2, lng2 float64) float64 {
	const earthRadiusMeters = 6371000.0
	toRad := func(deg float64) float64 {
		return deg * math.Pi / 180
	}
	dLat := toRad(lat2 - lat1)
	dLng := toRad(lng2 - lng1)
	a := math.Sin(dLat/2)*math.Sin(dLat/2) +
		math.Cos(toRad(lat1))*math.Cos(toRad(lat2))*
			math.Sin(dLng/2)*math.Sin(dLng/2)
	c := 2 * math.Atan2(math.Sqrt(a), math.Sqrt(1-a))
	return earthRadiusMeters * c
}

func tagOverlapRatio(source, target []string) float64 {
	sourceSet := make(map[string]struct{}, len(source))
	for _, s := range source {
		sourceSet[s] = struct{}{}
	}
	targetSet := make(map[string]struct{}, len(target))
	for _, t := range target {
		targetSet[t] = struct{}{}
	}

	intersection := 0
	unionSet := make(map[string]struct{}, len(sourceSet)+len(targetSet))
	for key := range sourceSet {
		unionSet[key] = struct{}{}
		if _, ok := targetSet[key]; ok {
			intersection++
		}
	}
	for key := range targetSet {
		unionSet[key] = struct{}{}
	}
	if len(unionSet) == 0 {
		return 0
	}
	return float64(intersection) / float64(len(unionSet))
}

func hasSharedTags(source, target []string) bool {
	targetSet := make(map[string]struct{}, len(target))
	for _, t := range target {
		targetSet[t] = struct{}{}
	}
	for _, s := range source {
		if _, ok := targetSet[s]; ok {
			return true
		}
	}
	return false
}

func recencyScore(createdAt, now time.Time) float64 {
	ageDays := now.Sub(createdAt).Hours() / 24
	return clamp01(1 - ageDays/float64(dedupeLookbackDays))
}

type dedupeCandidate struct {
	ReportID       int
	Score          float64
	DistanceMeters float64
}

func scoreDuplicateCandidate(incoming Report, candidate Report, now time.Time) *dedupeCandidate {
	distanceMeters := haversineMeters(incoming.Location.Lat, incoming.Location.Lng, candidate.Location.Lat, candidate.Location.Lng)
	if distanceMeters > dedupeRadiusMeters {
		return nil
	}

	distanceScore := clamp01(1 - distanceMeters/dedupeRadiusMeters)
	overlap := tagOverlapRatio(incoming.Tags, candidate.Tags)
	candidateCreated, _ := time.Parse(time.RFC3339, candidate.CreatedAt)
	recency := recencyScore(candidateCreated, now)

	score := distanceScore*distanceWeight + overlap*tagOverlapWeight + recency*recencyWeight
	return &dedupeCandidate{
		ReportID:       candidate.ID,
		Score:          math.Round(score*10000) / 10000,
		DistanceMeters: math.Round(distanceMeters*100) / 100,
	}
}

func scoreSignalGroupCandidate(incoming Report, candidate Report, now time.Time) *float64 {
	if !hasSharedTags(incoming.Tags, candidate.Tags) {
		return nil
	}
	distanceMeters := haversineMeters(incoming.Location.Lat, incoming.Location.Lng, candidate.Location.Lat, candidate.Location.Lng)
	if distanceMeters > signalMatchRadiusMeters {
		return nil
	}
	distanceScore := clamp01(1 - distanceMeters/signalMatchRadiusMeters)
	overlap := tagOverlapRatio(incoming.Tags, candidate.Tags)
	candidateCreated, _ := time.Parse(time.RFC3339, candidate.CreatedAt)
	recency := recencyScore(candidateCreated, now)
	score := distanceScore*distanceWeight + overlap*tagOverlapWeight + recency*recencyWeight
	rounded := math.Round(score*10000) / 10000
	return &rounded
}

func isSameUTCDay(leftISO, rightISO string) bool {
	left, errLeft := time.Parse(time.RFC3339, leftISO)
	right, errRight := time.Parse(time.RFC3339, rightISO)
	if errLeft != nil || errRight != nil {
		return false
	}
	ly, lm, ld := left.UTC().Date()
	ry, rm, rd := right.UTC().Date()
	return ly == ry && lm == rm && ld == rd
}

func computeSignalStrength(summary ReportSignalSummary) string {
	if summary.DistinctReporterReconfirmations > 0 && summary.UniqueReporters >= strongSignalMinReporters {
		return "strong_distinct_reporters"
	}
	if summary.HasQualifyingReconfirmation {
		return "weak_same_reporter"
	}
	return "none"
}

type reconfirmationComputation struct {
	Summary                  ReportSignalSummary
	SignalStrength           string
	ClassificationByReportID map[int]string
}

// effectiveReporterID returns a stable reporter identity string for signal grouping.
// Authenticated reporters are identified by user_id to correctly group reports
// from the same user across multiple devices or sessions.
func effectiveReporterID(r Report) string {
	if r.UserID != nil && *r.UserID != 0 {
		return "user:" + strconv.Itoa(*r.UserID)
	}
	return "anon:" + r.ReporterHash
}

func computeReconfirmation(reports []Report) reconfirmationComputation {
	sortedReports := append([]Report{}, reports...)
	sort.Slice(sortedReports, func(i, j int) bool {
		return sortedReports[i].CreatedAt < sortedReports[j].CreatedAt
	})

	classifications := make(map[int]string, len(sortedReports))
	sameReporterCount := 0
	distinctReporterCount := 0
	var firstQualifying *string
	var lastQualifying *string

	for i := range sortedReports {
		current := sortedReports[i]
		currentReporterID := effectiveReporterID(current)
		if i == 0 {
			classifications[current.ID] = "initial"
			continue
		}
		previousReports := sortedReports[:i]
		previous := previousReports[len(previousReports)-1]

		sameDayRepeatBySameReporter := false
		for _, candidate := range previousReports {
			if effectiveReporterID(candidate) == currentReporterID && isSameUTCDay(candidate.CreatedAt, current.CreatedAt) {
				sameDayRepeatBySameReporter = true
				break
			}
		}
		if sameDayRepeatBySameReporter {
			classifications[current.ID] = "ignored_same_day"
			continue
		}

		currentTime, _ := time.Parse(time.RFC3339, current.CreatedAt)
		previousTime, _ := time.Parse(time.RFC3339, previous.CreatedAt)
		gapDays := currentTime.Sub(previousTime).Hours() / 24
		if gapDays < float64(signalReconfirmationGapDays) {
			classifications[current.ID] = "non_qualifying"
			continue
		}

		priorReporterIDs := make(map[string]struct{}, len(previousReports))
		for _, report := range previousReports {
			priorReporterIDs[effectiveReporterID(report)] = struct{}{}
		}

		if _, ok := priorReporterIDs[currentReporterID]; ok {
			sameReporterCount++
			classifications[current.ID] = "counted_same_reporter"
		} else {
			distinctReporterCount++
			classifications[current.ID] = "counted_distinct_reporter"
		}

		if firstQualifying == nil {
			value := current.CreatedAt
			firstQualifying = &value
		}
		value := current.CreatedAt
		lastQualifying = &value
	}

	uniqueReportersSet := make(map[string]struct{}, len(sortedReports))
	for _, report := range sortedReports {
		uniqueReportersSet[effectiveReporterID(report)] = struct{}{}
	}

	lastReportAt := time.Now().UTC().Format(time.RFC3339)
	if len(sortedReports) > 0 {
		lastReportAt = sortedReports[len(sortedReports)-1].CreatedAt
	}

	summary := ReportSignalSummary{
		TotalReports:                    len(sortedReports),
		UniqueReporters:                 len(uniqueReportersSet),
		SameReporterReconfirmations:     sameReporterCount,
		DistinctReporterReconfirmations: distinctReporterCount,
		FirstQualifyingReconfirmationAt: firstQualifying,
		LastQualifyingReconfirmationAt:  lastQualifying,
		LastReportAt:                    lastReportAt,
		HasQualifyingReconfirmation:     sameReporterCount+distinctReporterCount > 0,
	}

	return reconfirmationComputation{
		Summary:                  summary,
		SignalStrength:           computeSignalStrength(summary),
		ClassificationByReportID: classifications,
	}
}

func applySummaryToBikeGroup(group BikeGroup, summary ReportSignalSummary, signalStrength string) BikeGroup {
	now := time.Now().UTC().Format(time.RFC3339)
	group.UpdatedAt = now
	group.LastReportAt = summary.LastReportAt
	group.TotalReports = summary.TotalReports
	group.UniqueReporters = summary.UniqueReporters
	group.SameReporterReconfirmations = summary.SameReporterReconfirmations
	group.DistinctReporterReconfirmations = summary.DistinctReporterReconfirmations
	group.FirstQualifyingReconfirmationAt = summary.FirstQualifyingReconfirmationAt
	group.LastQualifyingReconfirmationAt = summary.LastQualifyingReconfirmationAt
	group.SignalStrength = signalStrength
	return group
}

func bikeGroupToSignalSummary(group BikeGroup) ReportSignalSummary {
	return ReportSignalSummary{
		TotalReports:                    group.TotalReports,
		UniqueReporters:                 group.UniqueReporters,
		SameReporterReconfirmations:     group.SameReporterReconfirmations,
		DistinctReporterReconfirmations: group.DistinctReporterReconfirmations,
		FirstQualifyingReconfirmationAt: group.FirstQualifyingReconfirmationAt,
		LastQualifyingReconfirmationAt:  group.LastQualifyingReconfirmationAt,
		LastReportAt:                    group.LastReportAt,
		HasQualifyingReconfirmation:     group.SameReporterReconfirmations+group.DistinctReporterReconfirmations > 0,
	}
}

func buildPublicURL(baseURL, path string) string {
	if strings.HasPrefix(path, "/") {
		return strings.TrimRight(baseURL, "/") + path
	}
	return strings.TrimRight(baseURL, "/") + "/" + path
}

func (a *App) createTrackingToken(publicID string, expiresIn time.Duration) (string, error) {
	claims := jwt.MapClaims{
		"public_id": publicID,
		"iat":       time.Now().Unix(),
		"exp":       time.Now().Add(expiresIn).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.cfg.AppSigningSecret))
}

func (a *App) verifyTrackingToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(a.cfg.AppSigningSecret), nil
	})
	if err != nil || !token.Valid {
		return "", fmt.Errorf("invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", fmt.Errorf("invalid claims")
	}
	publicID, ok := claims["public_id"].(string)
	if !ok || publicID == "" {
		return "", fmt.Errorf("missing public_id")
	}
	return publicID, nil
}

func (a *App) createOperatorSessionToken(session OperatorSession) (string, error) {
	claims := jwt.MapClaims{
		"email": session.Email,
		"role":  session.Role,
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(operatorSessionDuration).Unix(),
	}
	if session.Municipality != nil {
		claims["municipality"] = *session.Municipality
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.cfg.AppSigningSecret))
}

func (a *App) verifyOperatorSessionToken(tokenString string) (*OperatorSession, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(a.cfg.AppSigningSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid session token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	email, _ := claims["email"].(string)
	role, _ := claims["role"].(string)
	if email == "" || !containsString(operatorRoles, role) {
		return nil, fmt.Errorf("invalid session payload")
	}
	session := &OperatorSession{Email: email, Role: role}
	if municipality, ok := claims["municipality"].(string); ok && municipality != "" {
		session.Municipality = &municipality
	}
	return session, nil
}

func (a *App) createUserSessionToken(session UserSession) (string, error) {
	claims := jwt.MapClaims{
		"user_id": session.UserID,
		"email":   session.Email,
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(userSessionDuration).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(a.cfg.AppSigningSecret))
}

func (a *App) verifyUserSessionToken(tokenString string) (*UserSession, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(a.cfg.AppSigningSecret), nil
	})
	if err != nil || !token.Valid {
		return nil, fmt.Errorf("invalid user session token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, fmt.Errorf("invalid token claims")
	}
	var userID int
	switch val := claims["user_id"].(type) {
	case float64:
		if val <= 0 || val != math.Trunc(val) {
			return nil, fmt.Errorf("invalid user_id claim")
		}
		userID = int(val)
	case string:
		id, convErr := strconv.Atoi(val)
		if convErr != nil || id <= 0 {
			return nil, fmt.Errorf("invalid user_id claim")
		}
		userID = id
	default:
		return nil, fmt.Errorf("invalid user_id claim")
	}

	email, _ := claims["email"].(string)
	if email == "" {
		return nil, fmt.Errorf("invalid user session payload")
	}
	return &UserSession{UserID: userID, Email: email}, nil
}

func createMagicLinkToken() string {
	return uuid.NewString()
}

func hashMagicLinkToken(token string) string {
	h := sha256.Sum256([]byte(token))
	return hex.EncodeToString(h[:])
}

func (a *App) deriveReporterHash(anonymousID string) string {
	h := sha256.Sum256([]byte(fmt.Sprintf("%s:%s", anonymousID, a.cfg.AppSigningSecret)))
	return hex.EncodeToString(h[:])
}

func generatePublicID() string {
	return strings.ToUpper(strings.ReplaceAll(uuid.NewString(), "-", "")[:8])
}

func parseTagsJSON(raw []byte) ([]string, error) {
	if len(raw) == 0 {
		return []string{}, nil
	}
	var tags []string
	if err := json.Unmarshal(raw, &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func tagsToJSON(tags []string) []byte {
	encoded, _ := json.Marshal(tags)
	return encoded
}

func anyMapToJSON(value map[string]any) []byte {
	encoded, _ := json.Marshal(value)
	return encoded
}

func jsonToAnyMap(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		return map[string]any{}
	}
	return decoded
}

func (a *App) checkRateLimit(key string, maxRequests int, window time.Duration, now time.Time) bool {
	a.rateLimiterMu.Lock()
	defer a.rateLimiterMu.Unlock()

	bucket, ok := a.rateBuckets[key]
	if !ok || now.Sub(bucket.start) >= window {
		a.rateBuckets[key] = rateBucket{start: now, count: 1}
		return true
	}
	bucket.count++
	a.rateBuckets[key] = bucket
	return bucket.count <= maxRequests
}

func (a *App) applyFingerprintHeuristic(fingerprintHash string, now time.Time) bool {
	a.fingerprintMu.Lock()
	defer a.fingerprintMu.Unlock()

	bucket, ok := a.fingerprints[fingerprintHash]
	if !ok || now.Sub(bucket.start) > reportRateLimitWindow {
		a.fingerprints[fingerprintHash] = fingerprintBucket{start: now, count: 1}
		return false
	}
	bucket.count++
	a.fingerprints[fingerprintHash] = bucket
	return bucket.count >= fingerprintBurstThreshold
}

func (a *App) startRateLimiterCleanup(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Minute
	}

	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				a.pruneRateLimiterState(now)
			}
		}
	}()
}

func (a *App) pruneRateLimiterState(now time.Time) {
	a.rateLimiterMu.Lock()
	for key, bucket := range a.rateBuckets {
		if now.Sub(bucket.start) >= reportRateLimitWindow {
			delete(a.rateBuckets, key)
		}
	}
	a.rateLimiterMu.Unlock()

	a.fingerprintMu.Lock()
	for key, bucket := range a.fingerprints {
		if now.Sub(bucket.start) >= reportRateLimitWindow {
			delete(a.fingerprints, key)
		}
	}
	a.fingerprintMu.Unlock()
}

func buildFingerprint(ip, userAgent, acceptLanguage string) string {
	normalized := fmt.Sprintf("%s|%s|%s", ip, valueOrDefaultString(userAgent, "na"), valueOrDefaultString(acceptLanguage, "na"))
	h := sha256.Sum256([]byte(normalized))
	return hex.EncodeToString(h[:])
}

func valueOrDefaultString(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (a *App) requireOperatorSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(operatorCookieName)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Operator session required"})
			c.Abort()
			return
		}
		session, err := a.verifyOperatorSessionToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Operator session required"})
			c.Abort()
			return
		}
		c.Set("operatorSession", *session)
		c.Next()
	}
}

func (a *App) requireOperatorSessionHTML() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(operatorCookieName)
		if err != nil {
			next := sanitizeAdminRedirectTarget(c.Request.URL.RequestURI())
			c.Redirect(http.StatusSeeOther, "/bikeadmin/login?next="+url.QueryEscape(next))
			c.Abort()
			return
		}
		session, err := a.verifyOperatorSessionToken(token)
		if err != nil {
			next := sanitizeAdminRedirectTarget(c.Request.URL.RequestURI())
			c.Redirect(http.StatusSeeOther, "/bikeadmin/login?next="+url.QueryEscape(next))
			c.Abort()
			return
		}
		c.Set("operatorSession", *session)
		c.Next()
	}
}

func (a *App) requireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		value, ok := c.Get("operatorSession")
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Operator session required"})
			c.Abort()
			return
		}
		session, ok := value.(OperatorSession)
		if !ok {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "Operator session required"})
			c.Abort()
			return
		}
		if session.Role != role {
			c.JSON(http.StatusForbidden, gin.H{"error": "forbidden", "message": "Insufficient role"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func getOperatorSession(c *gin.Context) (OperatorSession, error) {
	value, ok := c.Get("operatorSession")
	if !ok {
		return OperatorSession{}, fmt.Errorf("missing session")
	}
	session, ok := value.(OperatorSession)
	if !ok {
		return OperatorSession{}, fmt.Errorf("invalid session")
	}
	return session, nil
}

func (a *App) requireUserSession() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, err := c.Cookie(userCookieName)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "User session required"})
			c.Abort()
			return
		}
		session, err := a.verifyUserSessionToken(token)
		if err != nil {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized", "message": "User session required"})
			c.Abort()
			return
		}
		c.Set("userSession", *session)
		c.Next()
	}
}

func getUserSession(c *gin.Context) (UserSession, error) {
	value, ok := c.Get("userSession")
	if !ok {
		return UserSession{}, fmt.Errorf("missing user session")
	}
	session, ok := value.(UserSession)
	if !ok {
		return UserSession{}, fmt.Errorf("invalid user session")
	}
	return session, nil
}
