# SWITCH

Couple of years ago (back in early 2022), a friend asked me to give him a hand for a `Kotlin` game he was trying to code: A 3x3 grid of 2 states switches that, when toggled, change the state of their neighbours in a + shaped area around the aforementioned toggled switch.

Being a complete noob in `Kotlin`, I whipped up a [python draft](#python-draft) script in order to illustrate how I'd proceed...

Fast forward to 2024, after recieving some UIX advices from a former colleague and friend for a [side project](https://github.com/Luraminaki/pySET), he suggested me to give a try to `Go`... And for some reason, this game from 2022 just popped in my brain... So... Here goes nothing I guess...

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
    def __init__(self, cols, rows):
        self.rows = rows
        self.cols = cols
        self.grid = []
        self.rand = random.Random()

        self.init_game()
        while self.check_win():
            self.init_game()


    def init_game(self) -> None:
        self.grid = []
        for r in range(self.rows):
            temp_row = self.rand.choices([0, 1], k=self.cols)
            self.grid = self.grid + temp_row.copy()
            r = r + 1


    def check_win(self) -> bool:
        if sum(self.grid) in [0, self.rows*self.cols]:
            return True
        return False


    def check_oob(self, x: int, y: int) -> bool:
        if (0 <= x < self.cols) and (0 <= y < self.rows):
            return True
        return False


    def switch(self, x: int, y: int, neighborhood: list[int]) -> None:
        if not self.check_oob(x, y):
            return None

        coords_to_switch = []

        for val in neighborhood:
            if val == 0:
                coords_to_switch = coords_to_switch + [[x, y]]

            elif val == 4:
                coords_to_switch = coords_to_switch + self.switch_v4(x, y)

            elif val == 8:
                coords_to_switch = coords_to_switch + self.switch_v8(x, y)

            else:
                continue

        for cx, cy in coords_to_switch:
            self.grid[cx + self.cols*cy] = int(not self.grid[cx + self.cols*cy])


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


    def pretty_print_grid(self) -> None:
        print("Game Layout:")
        line = ""
        r = 0
        while r < self.rows:
            c = 0
            while c < self.cols:
                line = line + str(self.grid[c + self.cols*r]) + " "
                c = c + 1
            print(line)
            line = ""
            r = r + 1
        print("")


def main() -> None:
    switch_game = Grid(3, 3)
    switch_game.pretty_print_grid()

    while not switch_game.check_win():
        print("Input Col (x) Value")
        try:
            x = int(input())
        except Exception as err:
            print(f"Error when reading input value: {repr(err)}")
            continue

        print("Input Row (y) Value")
        try:
            y = int(input())
        except Exception as err:
            print(f"Error when reading input value: {repr(err)}")
            continue

        print(f"Switching ({x},{y})\n")
        switch_game.switch(x, y, [0, 4])
        switch_game.pretty_print_grid()
        print(f"Did I Win: {'Yes' if switch_game.check_win() else 'No'}")


if __name__ == "__main__":
    main()
```