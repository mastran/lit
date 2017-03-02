package wallit

import (
	"fmt"
	"log"
	"sort"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/mit-dci/lit/lnutil"
	"github.com/mit-dci/lit/portxo"
)

/*
type UWallet interface {
	GetPub(k portxo.KeyGen) *btcec.PublicKey

	GetPriv(k portxo.KeyGen) *btcec.PrivateKey

	PushTx(tx *wire.MsgTx) error
	ExportUtxo(txo *portxo.PorTxo)
	MaybeSend(txos []*wire.TxOut) ([]*wire.OutPoint, error)
	ReallySend(txid *chainhash.Hash) error
	NahDontSend(txid *chainhash.Hash) error
	WatchThis(wire.OutPoint) error
	LetMeKnow() chan lnutil.OutPointEvent
	BlockMonitor() chan *wire.MsgBlock

	Params() *chaincfg.Params
}
*/

// --- implementation of BaseWallet interface ----

func (w *Wallit) GetPriv(k portxo.KeyGen) *btcec.PrivateKey {
	return w.PathPrivkey(k)
}

func (w *Wallit) GetPub(k portxo.KeyGen) *btcec.PublicKey {
	return w.PathPubkey(k)
}

func (w *Wallit) PushTx(tx *wire.MsgTx) error {
	return w.Hook.PushTx(tx)
}

func (w *Wallit) Params() *chaincfg.Params {
	return w.Param
}

func (w *Wallit) BlockMonitor() chan *wire.MsgBlock {
	return w.Hook.RawBlocks()
}

func (w *Wallit) LetMeKnow() chan lnutil.OutPointEvent {
	w.OPEventChan = make(chan lnutil.OutPointEvent, 1)
	return w.OPEventChan
}

func (w *Wallit) CurrentHeight() int32 {
	h, err := w.GetDBSyncHeight()
	if err != nil {
		fmt.Printf("can't get height from db...")
		return -99
	}
	return h
}

func (w *Wallit) NewAdr() btcutil.Address {
	var a btcutil.Address
	adr160, err := w.NewAdr160()
	if err != nil {
		// should have an error here..?  Return empty address...
		fmt.Printf("can't make address: %s\n", err.Error())
		return a
	}

	a, err = btcutil.NewAddressWitnessPubKeyHash(adr160, w.Param)
	if err != nil {
		// should have an error here..?  Return empty address...
		fmt.Printf("can't make address: %s\n", err.Error())
	}

	return a
}

// ExportUtxo is really *IM*port utxo on this side.
// Not implemented yet.  Fix "ingest many" at the same time eh?
func (w *Wallit) ExportUtxo(u *portxo.PorTxo) {

	// zero value utxo counts as an address exort, not utxo export.
	if u.Value == 0 {
		err := w.AddPorTxoAdr(u.KeyGen)
		if err != nil {
			log.Printf(err.Error())
		}
	} else {
		err := w.GainUtxo(*u)
		if err != nil {
			log.Printf(err.Error())
		}
	}

	// Register new address with chainhook
	var adr160 [20]byte
	copy(adr160[:], w.PathPubHash160(u.KeyGen))
	err := w.Hook.RegisterAddress(adr160)
	if err != nil {
		log.Printf(err.Error())
	}
}

// WatchThis registers an outpoint to watch.  Register as watched OP, and
// passes to chainhook.
func (w *Wallit) WatchThis(op wire.OutPoint) error {
	err := w.Hook.RegisterOutPoint(op)
	if err != nil {
		return err
	}
	err = w.Hook.RegisterOutPoint(op)
	if err != nil {
		return err
	}
	return nil
}

// ********* sweep is for testing / spamming, remove for real use
func (w *Wallit) Sweep(adr btcutil.Address, n uint32) ([]*chainhash.Hash, error) {
	var err error
	var txids []*chainhash.Hash

	var utxos portxo.TxoSliceByAmt
	utxos, err = w.GetAllUtxos()
	if err != nil {
		return nil, err
	}

	// smallest and unconfirmed last (because it's reversed)
	sort.Sort(sort.Reverse(utxos))

	for _, u := range utxos {
		if n < 1 {
			return txids, nil
		}

		// this doesn't really work with maybeSend huh...
		if u.Height != 0 && u.Value > 20000 {
			outputscript, err := txscript.PayToAddrScript(adr)
			if err != nil {
				return nil, err
			}

			txo := wire.NewTxOut(u.Value-20000, outputscript)

			ops, err := w.MaybeSend([]*wire.TxOut{txo})
			if err != nil {
				return nil, err
			}

			err = w.ReallySend(&ops[0].Hash)
			if err != nil {
				return nil, err
			}
			txids = append(txids, &ops[0].Hash)
			n--
		}
	}

	return txids, nil
}