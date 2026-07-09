package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"clawreef/internal/models"
)

func TestParseRuntimeAgentSkillsReport(t *testing.T) {
	payload := map[string]any{
		"mode": "full",
		"instances": []map[string]any{
			{
				"instance_id": 12,
				"skills": []map[string]any{
					{
						"identifier":  "weather",
						"content_md5": "abc123",
						"source":      "runtime",
					},
				},
			},
		},
	}
	reports, mode, _, err := parseRuntimeAgentSkillsReport(payload)
	if err != nil {
		t.Fatalf("parseRuntimeAgentSkillsReport() error = %v", err)
	}
	if mode != "full" {
		t.Fatalf("mode = %q, want full", mode)
	}
	if len(reports) != 1 || reports[0].InstanceID != 12 {
		t.Fatalf("unexpected reports: %#v", reports)
	}
	if len(reports[0].Skills) != 1 || reports[0].Skills[0].Identifier != "weather" {
		t.Fatalf("unexpected skills: %#v", reports[0].Skills)
	}
}

func TestNormalizeRuntimeSkillSource(t *testing.T) {
	if got := normalizeRuntimeSkillSource("runtime"); got != "discovered_in_instance" {
		t.Fatalf("normalizeRuntimeSkillSource(runtime) = %q", got)
	}
	if got := normalizeRuntimeSkillSource("injected_by_clawmanager"); got != "injected_by_clawmanager" {
		t.Fatalf("normalizeRuntimeSkillSource(injected) = %q", got)
	}
}

func TestCollectLiteSkillDirectoryFiles(t *testing.T) {
	root := t.TempDir()
	skillRoot := filepath.Join(root, "weather")
	if err := os.MkdirAll(filepath.Join(skillRoot, "src"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("# weather\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "src", "main.py"), []byte("print('ok')\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	files, err := collectLiteSkillDirectoryFiles(skillRoot)
	if err != nil {
		t.Fatalf("collectLiteSkillDirectoryFiles() error = %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %#v, want 2 entries", files)
	}
	if got := hashDirectory(files); got == "" {
		t.Fatal("expected non-empty content md5")
	}
}

func TestCollectLiteSkillDirectoryFilesSkipsWithoutManifest(t *testing.T) {
	root := t.TempDir()
	skillRoot := filepath.Join(root, "orphan")
	if err := os.MkdirAll(skillRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	files, err := collectLiteSkillDirectoryFiles(skillRoot)
	if err != nil {
		t.Fatalf("collectLiteSkillDirectoryFiles() error = %v", err)
	}
	if files != nil {
		t.Fatalf("files = %#v, want nil", files)
	}
}

func TestDiscoverRuntimeSkillDirectoriesNestedCategory(t *testing.T) {
	root := t.TempDir()
	categoryRoot := filepath.Join(root, "productivity")
	skillRoot := filepath.Join(categoryRoot, "my-skill")
	for _, dir := range []string{skillRoot, filepath.Join(skillRoot, "src")} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "SKILL.md"), []byte("# my skill\n"), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillRoot, "src", "main.py"), []byte("print('ok')\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	discoveries, err := discoverRuntimeSkillDirectories(root, runtimeSkillDiscoveryMaxDepth)
	if err != nil {
		t.Fatalf("discoverRuntimeSkillDirectories() error = %v", err)
	}
	if len(discoveries) != 1 {
		t.Fatalf("discoveries = %#v, want 1 nested skill", discoveries)
	}
	if discoveries[0].RelativePath != "productivity/my-skill" {
		t.Fatalf("RelativePath = %q, want productivity/my-skill", discoveries[0].RelativePath)
	}
}

func TestDiscoverRuntimeSkillDirectoriesFlatAndNested(t *testing.T) {
	root := t.TempDir()
	flatRoot := filepath.Join(root, "weather")
	if err := os.MkdirAll(flatRoot, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(flatRoot, "SKILL.md"), []byte("# weather\n"), 0o640); err != nil {
		t.Fatal(err)
	}

	discoveries, err := discoverRuntimeSkillDirectories(root, runtimeSkillDiscoveryMaxDepth)
	if err != nil {
		t.Fatalf("discoverRuntimeSkillDirectories() error = %v", err)
	}
	if len(discoveries) != 1 || discoveries[0].RelativePath != "weather" {
		t.Fatalf("discoveries = %#v, want flat weather skill", discoveries)
	}
}

func TestRuntimeSkillInstallRootOpenClawAndHermes(t *testing.T) {
	workspace := "/workspaces/demo/instance-1"
	hermes := &models.Instance{Type: RuntimeTypeHermes, WorkspacePath: &workspace}
	openclaw := &models.Instance{Type: RuntimeTypeOpenClaw, WorkspacePath: &workspace}

	hermesRoot := runtimeSkillInstallRoot(hermes)
	openclawRoot := runtimeSkillInstallRoot(openclaw)
	if !strings.HasSuffix(filepath.ToSlash(hermesRoot), "home/.hermes/skills") {
		t.Fatalf("hermes root = %q", hermesRoot)
	}
	if !strings.HasSuffix(filepath.ToSlash(openclawRoot), "home/.openclaw/workspace/skills") {
		t.Fatalf("openclaw root = %q", openclawRoot)
	}
}

func TestResolveInstanceSkillSourceTypePreservesInjected(t *testing.T) {
	existing := &models.InstanceSkill{SourceType: "injected_by_clawmanager"}
	skill := &models.Skill{SourceType: skillSourceUploaded}
	got := resolveInstanceSkillSourceType(existing, "discovered_in_instance", skill)
	if got != "injected_by_clawmanager" {
		t.Fatalf("resolveInstanceSkillSourceType() = %q, want injected_by_clawmanager", got)
	}
}

func TestSanitizeWorkspaceRelativePath(t *testing.T) {
	if got := sanitizeWorkspaceRelativePath("productivity/my-skill"); got != "productivity/my-skill" {
		t.Fatalf("sanitizeWorkspaceRelativePath() = %q", got)
	}
	if got := sanitizeWorkspaceRelativePath("../escape"); got != "" {
		t.Fatalf("sanitizeWorkspaceRelativePath(../escape) = %q, want empty", got)
	}
}

func TestIsLiteRuntimeInstanceOpenClawVariants(t *testing.T) {
	liteGateway := &models.Instance{InstanceMode: InstanceModeLite, RuntimeType: RuntimeBackendGateway, Type: RuntimeTypeOpenClaw}
	if !IsLiteRuntimeInstance(liteGateway) {
		t.Fatal("expected gateway lite instance")
	}
	proDesktop := &models.Instance{InstanceMode: InstanceModePro, RuntimeType: RuntimeBackendDesktop, Type: RuntimeTypeOpenClaw}
	if IsLiteRuntimeInstance(proDesktop) {
		t.Fatal("expected pro desktop instance to be non-lite")
	}
	shellPod := &models.Instance{InstanceMode: InstanceModePro, RuntimeType: RuntimeBackendShell, Type: RuntimeTypeOpenClaw}
	if IsLiteRuntimeInstance(shellPod) {
		t.Fatal("expected shell pod instance to use pro agent path")
	}
}

type provenanceCaptureRepoStub struct {
	capturingSkillRepoStub
	upserted []*models.InstanceSkill
}

func (s *provenanceCaptureRepoStub) GetBlobByContentHash(hash string) (*models.SkillBlob, error) {
	for _, blob := range s.blobs {
		if strings.EqualFold(strings.TrimSpace(blob.ContentHash), strings.TrimSpace(hash)) {
			copy := *blob
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *provenanceCaptureRepoStub) GetVersionBySkillAndBlob(skillID, blobID int) (*models.SkillVersion, error) {
	for _, version := range s.versions {
		if version.SkillID == skillID && version.BlobID == blobID {
			copy := *version
			return &copy, nil
		}
	}
	return nil, nil
}

func (s *provenanceCaptureRepoStub) UpsertInstanceSkill(item *models.InstanceSkill) error {
	copy := *item
	s.upserted = append(s.upserted, &copy)
	updated := false
	for i, existing := range s.instanceSkills {
		if existing.InstanceID == item.InstanceID && existing.SkillID == item.SkillID {
			s.instanceSkills[i] = copy
			updated = true
			break
		}
	}
	if !updated {
		s.instanceSkills = append(s.instanceSkills, copy)
	}
	return nil
}

func TestSyncAgentSkillsPreservesInjectedProvenanceAfterWorkspaceScan(t *testing.T) {
	contentHash := "abc123def456789012345678901234"
	versionID := 1
	stub := &provenanceCaptureRepoStub{
		capturingSkillRepoStub: capturingSkillRepoStub{
			skillRepoStub: skillRepoStub{
			skills: map[int]*models.Skill{
				10: {
					ID: 10, UserID: 1, SkillKey: "ppt-1-0-0", Name: "ppt-1.0.0",
					SourceType: skillSourceUploaded, Status: skillStatusActive,
					Visibility: skillVisibilityPublic, CurrentVersionID: &versionID,
				},
			},
			blobs: map[int]*models.SkillBlob{
				1: {ID: 1, ContentHash: contentHash, ObjectKey: "hub/ppt.zip", ScanStatus: "completed"},
			},
			versions: map[int]*models.SkillVersion{
				1: {ID: 1, SkillID: 10, BlobID: 1, VersionNo: 1},
			},
			instanceSkills: []models.InstanceSkill{
				{InstanceID: 1, SkillID: 10, SourceType: "injected_by_clawmanager", Status: "active"},
			},
			},
		},
	}
	instRepo := &importTestInstanceRepo{instances: map[int]*models.Instance{
		1: {ID: 1, UserID: 1, Type: RuntimeTypeHermes, InstanceMode: InstanceModePro, RuntimeType: RuntimeBackendDesktop},
	}}
	svc := &skillService{repo: stub, instanceRepo: instRepo, commandService: &noopInstanceCommandService{}}
	err := svc.SyncAgentSkills(1, AgentSkillInventoryReportRequest{
		Mode: "full",
		Skills: []AgentSkillRecord{{
			Identifier:  "ppt-1-0-0",
			ContentMD5:  contentHash,
			Source:      "discovered_in_instance",
			InstallPath: "home/.hermes/skills/ppt-1-0-0",
		}},
	})
	if err != nil {
		t.Fatalf("SyncAgentSkills() error = %v", err)
	}
	if len(stub.upserted) != 1 {
		t.Fatalf("upserted %d instance skills, want 1", len(stub.upserted))
	}
	if stub.upserted[0].SourceType != "injected_by_clawmanager" {
		t.Fatalf("SourceType = %q, want injected_by_clawmanager", stub.upserted[0].SourceType)
	}
}

func TestSyncAgentSkillsReusesUploadedSkillOnWorkspaceScan(t *testing.T) {
	contentHash := "abc123def456789012345678901234"
	versionID := 1
	stub := &provenanceCaptureRepoStub{
		capturingSkillRepoStub: capturingSkillRepoStub{
			skillRepoStub: skillRepoStub{
			skills: map[int]*models.Skill{
				10: {
					ID: 10, UserID: 1, SkillKey: "ppt-1-0-0", Name: "ppt-1.0.0",
					SourceType: skillSourceUploaded, Status: skillStatusActive,
					Visibility: skillVisibilityPublic, CurrentVersionID: &versionID,
				},
			},
			blobs: map[int]*models.SkillBlob{
				1: {ID: 1, ContentHash: contentHash, ObjectKey: "hub/ppt.zip", ScanStatus: "completed"},
			},
			versions: map[int]*models.SkillVersion{
				1: {ID: 1, SkillID: 10, BlobID: 1, VersionNo: 1},
			},
			},
		},
	}
	instRepo := &importTestInstanceRepo{instances: map[int]*models.Instance{
		1: {ID: 1, UserID: 1, Type: RuntimeTypeHermes, InstanceMode: InstanceModeLite, RuntimeType: RuntimeBackendGateway},
	}}
	svc := &skillService{repo: stub, instanceRepo: instRepo, commandService: &noopInstanceCommandService{}}
	err := svc.SyncAgentSkills(1, AgentSkillInventoryReportRequest{
		Mode: "full",
		Skills: []AgentSkillRecord{{
			Identifier:  "ppt-1-0-0",
			ContentMD5:  contentHash,
			Source:      "discovered_in_instance",
			InstallPath: "home/.hermes/skills/ppt-1-0-0",
		}},
	})
	if err != nil {
		t.Fatalf("SyncAgentSkills() error = %v", err)
	}
	if len(stub.createdSkills) != 0 {
		t.Fatalf("created %d discovered skills, want 0 reuse of uploaded skill", len(stub.createdSkills))
	}
	if len(stub.upserted) != 1 || stub.upserted[0].SkillID != 10 {
		t.Fatalf("upserted = %#v, want instance skill for uploaded skill id 10", stub.upserted)
	}
}
