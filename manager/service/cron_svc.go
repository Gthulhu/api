package service

import "context"

func (svc *Service) CheckDMIntents(ctx context.Context) error {
	// TODO: kubectl get dm nodes

	// TODO: get all intents order by id from DB => merkle tree

	// TODO: compare with DM nodes' merkle tree root hash

	// TODO: report any discrepancies
	return nil
}
