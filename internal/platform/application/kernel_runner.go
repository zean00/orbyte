package application

import "context"

type KernelCommand[T any] interface {
	Run(context.Context, UnitOfWork) (T, error)
}

type KernelCommandRunner struct {
	txm TransactionManager
}

func NewKernelCommandRunner(txm TransactionManager) *KernelCommandRunner {
	return &KernelCommandRunner{txm: txm}
}

func RunKernelCommand[T any](ctx context.Context, r *KernelCommandRunner, cmd KernelCommand[T]) (T, error) {
	var zero T
	if r == nil || r.txm == nil {
		return zero, nil
	}
	var out T
	err := r.txm.WithinTx(ctx, func(uow UnitOfWork) error {
		var err error
		out, err = cmd.Run(ctx, uow)
		return err
	})
	if err != nil {
		return zero, err
	}
	return out, nil
}
