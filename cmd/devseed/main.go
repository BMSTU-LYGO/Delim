// Command devseed creates a disposable, local-only group for product demos.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"delim/internal/devtools"
	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	gatewayconfig "delim/internal/gateway/config"
	corev1 "delim/pkg/gen/core/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	ownerMAXID  = int64(9_082_000_001)
	memberMAXID = int64(9_082_000_002)
	guestMAXID  = int64(9_082_000_003)
)

type demoExpense struct {
	amountMinor  int64
	confirmed    bool
	description  string
	payerID      int64
	participants []int64
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	flags := flag.NewFlagSet("devseed", flag.ContinueOnError)
	coreAddress := flags.String("core-addr", "localhost:50051", "Core gRPC address")
	frontendEnv := flags.String(
		"frontend-env", "web/miniapp/.env.local", "generated Vite environment file",
	)
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: devseed [--core-addr address] [--frontend-env path]")
	}
	if err := devtools.LoadLocalEnv(".env"); err != nil {
		return err
	}
	cfg, err := gatewayconfig.Load("configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("load Gateway config: %w", err)
	}
	if err := devtools.RequireLocal(cfg.App.Env); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := coreclient.New(*coreAddress, nil)
	if err != nil {
		return fmt.Errorf("connect to Core: %w", err)
	}
	defer client.Close()
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("Core dev stack is not ready: %w", err)
	}

	sessions := auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	owner, err := devtools.ProvisionActor(
		ctx, client, sessions, ownerMAXID, "Мария", "maria_demo",
	)
	if err != nil {
		return err
	}
	member, err := devtools.ProvisionActor(
		ctx, client, sessions, memberMAXID, "Алексей", "alexey_demo",
	)
	if err != nil {
		return err
	}
	guest, err := devtools.ProvisionActor(
		ctx, client, sessions, guestMAXID, "Елена", "elena_demo",
	)
	if err != nil {
		return err
	}

	createdGroup, err := client.CreateGroup(ctx, &corev1.CreateGroupRequest{
		ActorUserId: owner.UserID,
		Name:        fmt.Sprintf("Демо: выходные %s", time.Now().Format("02.01 15:04")),
	})
	if err != nil {
		return fmt.Errorf("create demo group: %w", err)
	}
	groupID := createdGroup.GetGroup().GetId()
	if groupID <= 0 {
		return fmt.Errorf("create demo group: Core returned an invalid group")
	}
	if _, err := client.AddGroupMembers(ctx, &corev1.AddGroupMembersRequest{
		ActorUserId: owner.UserID,
		GroupId:     groupID,
		UserIds:     []int64{member.UserID, guest.UserID},
	}); err != nil {
		return fmt.Errorf("add demo members: %w", err)
	}

	all := []int64{owner.UserID, member.UserID, guest.UserID}
	expenses := []demoExpense{
		{amountMinor: 630_000, confirmed: true, description: "Ужин в ресторане", payerID: owner.UserID, participants: all},
		{amountMinor: 180_000, confirmed: true, description: "Такси", payerID: member.UserID, participants: []int64{owner.UserID, member.UserID}},
		{amountMinor: 450_000, confirmed: true, description: "Билеты в музей", payerID: guest.UserID, participants: all},
		{amountMinor: 240_000, description: "Продукты на завтрак", payerID: member.UserID, participants: all},
	}
	for index, expense := range expenses {
		if err := createExpense(ctx, client, owner.UserID, groupID, index, expense); err != nil {
			return err
		}
	}
	if err := writeFrontendEnv(*frontendEnv, owner.Token); err != nil {
		return err
	}

	fmt.Printf("Демо-группа #%d создана для Марии, Алексея и Елены.\n", groupID)
	fmt.Printf("Dev-сессия записана в %s.\n", *frontendEnv)
	fmt.Println("Запуск интерфейса: npm --prefix web/miniapp run dev")
	return nil
}

func createExpense(
	ctx context.Context,
	client *coreclient.Client,
	actorID int64,
	groupID int64,
	index int,
	value demoExpense,
) error {
	participants := make([]*corev1.SplitParticipant, 0, len(value.participants))
	for _, userID := range value.participants {
		participants = append(participants, &corev1.SplitParticipant{UserId: userID})
	}
	created, err := client.CreateExpense(ctx, &corev1.CreateExpenseRequest{
		ActorUserId: actorID,
		Expense: &corev1.ExpenseInput{
			AmountMinor:  value.amountMinor,
			Currency:     "RUB",
			Description:  value.description,
			ExpenseDate:  timestamppb.New(time.Now().Add(time.Duration(-72+index*12) * time.Hour)),
			GroupId:      groupID,
			Participants: participants,
			PayerUserId:  value.payerID,
			SplitType:    corev1.SplitType_SPLIT_TYPE_EQUAL,
		},
	})
	if err != nil {
		return fmt.Errorf("create demo expense %q: %w", value.description, err)
	}
	if !value.confirmed {
		return nil
	}
	if _, err := client.ConfirmExpense(ctx, &corev1.ConfirmExpenseRequest{
		ActorUserId: actorID,
		ExpenseId:   created.GetExpense().GetId(),
	}); err != nil {
		return fmt.Errorf("confirm demo expense %q: %w", value.description, err)
	}
	return nil
}

func writeFrontendEnv(path, token string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create frontend environment directory: %w", err)
	}
	content := []byte(
		"# Generated by make demo-seed. Local development only.\n" +
			"VITE_GATEWAY_URL=http://localhost:8080\n" +
			"VITE_DEV_SESSION_TOKEN=" + token + "\n",
	)
	if err := os.WriteFile(path, content, 0o600); err != nil {
		return fmt.Errorf("write frontend environment: %w", err)
	}
	return nil
}
