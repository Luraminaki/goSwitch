# SWITCH

Couple of years ago (back in early 2022), a friend asked me to give him a hand for a `Kotlin` game he was trying to code: A 3x3 grid of 2 states switches that, when toggled, change the state of their neighbours in a + shaped area around the aforementioned toggled switch. (I later learned that the game is called [Lights out](https://en.wikipedia.org/wiki/Lights_Out_(game)))

Being a complete noob in `Kotlin`, I whipped up a [python draft](#python-draft) script in order to illustrate how I'd proceed...

Fast forward to 2024, after recieving some UIX advices from a former colleague and friend for a [side project](https://github.com/Luraminaki/pySET), he suggested me to give a try to `Go` and `HTMX`... And for some reason, this game from 2022 just popped in my brain... So... Here `Go`es nothing I guess... :D

_although some parts could have been a lot simpler, had I used some `JS`, I feel that it would have defeated the whole purpose of this project which was to stick to `Go` and `HTMX` as much as possible..._

## VERSIONS

- 0.1.0-alpha: First release

## TABLE OF CONTENT

<!-- TOC -->

- [SWITCH](#switch)
  - [VERSIONS](#versions)
  - [TABLE OF CONTENT](#table-of-content)
  - [INSTALL AND RUN](#install-and-run)
  - [PYTHON DRAFT](#python-draft)

<!-- /TOC -->

## INSTALL AND RUN

For `Go` installation, consult the following [link](https://go.dev/)

Once done, open a new terminal in the directory `goSwitch` and type the following command to run the project:

```sh
go run .
```

If compiling the code into an executable is what you are looking for, in the same fashion as mentionned above, type the following command:

```sh
go build
```

## PYTHON DRAFT

```py
#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
Created on Mon Jan 31 22:59:51 2022

@author: Luraminaki
"""

#===================================================================================================
import random

#pylint: disable=wrong-import-order, wrong-import-position

#pylint: enable=wrong-import-order, wrong-import-position
#===================================================================================================


__version__ = '0.1.0'


class Grid():
    def __init__(self, dim: int, neighborhood: list[int]):
        self._rows = dim
        self._cols = dim
        self._neighborhood = neighborhood

        self._grid: list[int] = []
        self._solution: list[int] = [] # Not necessarily the fastest

        self._rand = random.Random()

        self.init_game()
        while self.check_win():
            self.init_game()


    def init_game(self) -> None:
        grid_size = self._rows*self._cols
        start = self._rand.choice(range(1))

        self._grid = [start] * grid_size
        self._solution = self._rand.sample(list(range(grid_size)),
                                           k=self._rand.choice(range(grid_size)) + 1)
        self._solution.sort()

        for hit in self._solution:
            self.switch(hit)


    def coord_flat_to_cart(self, dim: int) -> tuple[int]:
        if dim >= len(self._grid):
            return -1, -1
        return (dim % self._cols, dim // self._rows)


    def check_oob(self, x: int, y: int) -> bool:
        if (0 <= x < self._cols) and (0 <= y < self._rows):
            return True
        return False


    def switch_v4(self, x: int, y: int) -> list[list[int]] | list:
        coords_to_switch = []

        if self.check_oob(x + 1, y):
            coords_to_switch.append([x+1, y])
        if self.check_oob(x, y+1):
            coords_to_switch.append([x, y+1])
        if self.check_oob(x - 1, y):
            coords_to_switch.append([x-1, y])
        if self.check_oob(x, y - 1):
            coords_to_switch.append([x, y-1])

        return coords_to_switch


    def switch_v8(self, x: int, y: int) -> list[list[int]] | list:
        coords_to_switch = []

        if self.check_oob(x + 1, y + 1):
            coords_to_switch.append([x+1, y+1])
        if self.check_oob(x - 1, y - 1):
            coords_to_switch.append([x-1, y-1])
        if self.check_oob(x + 1, y - 1):
            coords_to_switch.append([x+1, y-1])
        if self.check_oob(x - 1, y + 1):
            coords_to_switch.append([x-1, y+1])

        return coords_to_switch


    def switch(self, pos: int) -> None:
        x, y = self.coord_flat_to_cart(pos)

        if not self.check_oob(x, y):
            return None

        coords_to_switch = []

        for val in self._neighborhood:
            if val == 0:
                coords_to_switch = coords_to_switch + [[x, y]]

            elif val == 4:
                coords_to_switch = coords_to_switch + self.switch_v4(x, y)

            elif val == 8:
                coords_to_switch = coords_to_switch + self.switch_v8(x, y)

            else:
                continue

        for cx, cy in coords_to_switch:
            self._grid[cx + self._cols*cy] = int(not self._grid[cx + self._cols*cy])


    def get_possible_solution(self) -> list[int]:
        return self._solution.copy()


    def get_grid(self) -> list[int]:
        return self._grid.copy()


    def check_win(self) -> bool:
        if sum(self._grid) in [0, self._rows*self._cols]:
            return True
        return False


    def pretty_print_grid(self) -> None:
        print("Game Layout:")
        line = ""
        r = 0
        while r < self._rows:
            c = 0
            while c < self._cols:
                line = line + str(self._grid[c + self._cols*r]) + " "
                c = c + 1
            print(line)
            line = ""
            r = r + 1
        print("")


def main() -> None:
    dim = 3
    switch_game = Grid(dim, [0, 4])

    print(f"Possible solution: {switch_game.get_possible_solution()}")

    switch_game.pretty_print_grid()

    while not switch_game.check_win():
        print(f"Input Switch Position (0 ~ {(dim*dim)-1}):")
        try:
            pos = int(input())
        except Exception as err:
            print(f"Error when reading input value: {repr(err)}")
            continue

        print(f"Switching ({pos})\n")
        switch_game.switch(pos)
        switch_game.pretty_print_grid()
        print(f"Did I Win: {'Yes' if switch_game.check_win() else 'No'}")


if __name__ == "__main__":
    main()
```