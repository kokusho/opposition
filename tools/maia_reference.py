"""Writes reference predictions from the official Maia-3 PyTorch model.

The Go implementation in internal/maia runs an ONNX conversion of the model
and re-implements the input encoding. Its tests compare against this file's
output, so a mistake in either shows up as a mismatch.

Needs the official package (https://github.com/CSSLab/maia3) installed:

    .maia3/venv/bin/python tools/maia_reference.py maia3-5m > testdata/maia/reference-5m.json
"""

import json
import sys

import chess
import torch

from maia3.dataset import get_legal_moves_mask, tokenize_board
from maia3.uci import Maia3UCIEngine, parse_args
from maia3.utils import mirror_move

# (name, FEN, rating of the side to move, rating of the other side)
CASES = [
    ("start, club players", chess.STARTING_FEN, 1500, 1500),
    ("start, beginners", chess.STARTING_FEN, 800, 800),
    ("start, masters", chess.STARTING_FEN, 2400, 2400),
    ("black to move after 1.e4", "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq - 0 1", 1500, 1500),
    ("black after 1.e4 e5 2.Nf3, weak mover", "rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2", 1000, 1000),
    ("black after 1.e4 e5 2.Nf3, strong mover", "rnbqkbnr/pppp1ppp/8/4p3/4P3/5N2/PPPP1PPP/RNBQKB1R b KQkq - 1 2", 2200, 2200),
    ("white can castle, unequal ratings", "r1bqk1nr/pppp1ppp/2n5/2b1p3/2B1P3/5N2/PPPP1PPP/RNBQK2R w KQkq - 4 4", 1800, 1300),
    ("black can castle, unequal ratings", "rnbqk2r/pppp1ppp/5n2/2b1p3/2B1P3/5N2/PPPP1PPP/RNBQ1RK1 b kq - 5 4", 1300, 1800),
    ("white can capture en passant", "rnbqkbnr/ppp1p1pp/8/3pPp2/8/8/PPPP1PPP/RNBQKBNR w KQkq f6 0 3", 1500, 1500),
    ("white promotes", "8/P6k/8/8/8/8/6K1/8 w - - 0 1", 1500, 1500),
    ("black promotes", "8/6k1/8/8/8/8/p6K/8 b - - 0 1", 1500, 1500),
    ("black promotes by capture", "8/6k1/8/8/8/8/1p5K/R7 b - - 0 1", 1900, 1900),
    ("middlegame, black to move", "r2q1rk1/pp2bppp/2n1bn2/3p4/3N4/2N1B3/PPPQBPPP/R4RK1 b - - 5 11", 2000, 2000),
]


def main():
    model = sys.argv[1] if len(sys.argv) > 1 else "maia3-5m"
    engine = Maia3UCIEngine(parse_args(["--model", model, "--device", "cpu", "--no-use-amp"]))
    engine.ensure_model_loaded()

    cases = []
    with torch.no_grad():
        for name, fen, self_elo, oppo_elo in CASES:
            board = chess.Board(fen)
            engine.board = board
            engine._reset_history()  # the current position repeated, as without move history

            tokens = engine._tokens_from_history(engine.history).unsqueeze(0)
            logits_move, logits_value, _ = engine.model(
                tokens,
                torch.tensor([self_elo], dtype=torch.long),
                torch.tensor([oppo_elo], dtype=torch.long),
            )

            mask = get_legal_moves_mask(board, engine.all_moves_dict)
            probs = torch.softmax(logits_move[0].float().masked_fill(~mask, float("-inf")), dim=-1)
            moves = {}
            for move in board.legal_moves:
                uci = move.uci()
                seen = uci if board.turn == chess.WHITE else mirror_move(uci)
                moves[uci] = round(float(probs[engine.all_moves_dict[seen]]), 7)

            loss, draw, win = torch.softmax(logits_value[0].float(), dim=-1).tolist()
            cases.append({
                "name": name,
                "fen": fen,
                "selfElo": self_elo,
                "oppoElo": oppo_elo,
                "moves": dict(sorted(moves.items(), key=lambda item: -item[1])),
                "win": round(win, 7),
                "draw": round(draw, 7),
                "loss": round(loss, 7),
                # The board encoding itself, as the indices of the set bits, to
                # test the encoder apart from the network.
                "tokens": [int(i) for i in tokenize_board(board).flatten().nonzero().flatten().tolist()],
            })

    json.dump({"model": model, "cases": cases}, sys.stdout, indent=1)
    sys.stdout.write("\n")


if __name__ == "__main__":
    main()
