package com.example.app;

import java.util.List;
import javax.swing.JFrame;
import javax.servlet.http.HttpServlet;
import com.example.app.model.User;
import com.example.app.model.*;
import static com.example.app.util.Strings.trim;
import com.example.generated.Gen;
import org.springframework.context.ApplicationContext;
import com.fasterxml.jackson.databind.ObjectMapper;
import org.slf4j.Logger;
import static org.junit.jupiter.api.Assertions.assertEquals;
import okhttp3.OkHttpClient;
import com.google.common.collect.Lists;
import org.junit.Test;
import org.acme.net.Client;

public class App {
    public static void main(String[] args) {}
    private void helper() {}
}

interface Service { void run(); }

enum Mode {
    A;
    int weight() { return 1; }
}

record Point(int x, int y) {}
