import 'package:flutter/material.dart';
import 'api_client.dart';
import 'dettaglio_screen.dart';

void main() {
  runApp(const LastMileApp());
}

class LastMileApp extends StatelessWidget {
  const LastMileApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Last Mile Delivery',
      debugShowCheckedModeBanner: false,
      theme: ThemeData(
        colorScheme: ColorScheme.fromSeed(seedColor: Colors.deepOrange),
        useMaterial3: true,
      ),
      home: const SpedizioniListScreen(),
    );
  }
}

class SpedizioniListScreen extends StatefulWidget {
  const SpedizioniListScreen({super.key});

  @override
  State<SpedizioniListScreen> createState() => _SpedizioniListScreenState();
}

class _SpedizioniListScreenState extends State<SpedizioniListScreen> {
  late Future<List<dynamic>> _spedizioniFuture;

  @override
  void initState() {
    super.initState();
    _ricaricaSpedizioni();
  }

  void _ricaricaSpedizioni() {
    setState(() {
      _spedizioniFuture = apiClient.getSpedizioni();
    });
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: const Text('Consegne Last Mile'),
        actions: [
          IconButton(
            icon: const Icon(Icons.refresh),
            onPressed: _ricaricaSpedizioni,
          )
        ],
      ),
      body: FutureBuilder<List<dynamic>>(
        future: _spedizioniFuture,
        builder: (context, snapshot) {
          if (snapshot.connectionState == ConnectionState.waiting) {
            return const Center(child: CircularProgressIndicator());
          }

          if (snapshot.hasError) {
            return Center(
              child: SingleChildScrollView(
                child: Padding(
                  padding: const EdgeInsets.all(16.0),
                  child: Column(
                    mainAxisAlignment: MainAxisAlignment.center,
                    children: [
                      const Icon(Icons.error_outline, color: Colors.red, size: 48),
                      const SizedBox(height: 12),
                      Text('Errore: ${snapshot.error}', textAlign: TextAlign.center),
                      const SizedBox(height: 12),
                      ElevatedButton(onPressed: _ricaricaSpedizioni, child: const Text('Riprova')),
                    ],
                  ),
                ),
              ),
            );
          }

          final spedizioni = snapshot.data ?? [];

          if (spedizioni.isEmpty) {
            return const Center(child: Text('Nessuna spedizione trovata.'));
          }

          return ListView.builder(
            itemCount: spedizioni.length,
            itemBuilder: (context, index) {
              final item = spedizioni[index];
              final stato = item['stato_corrente'] ?? item['stato'] ?? 'In attesa';

              return Card(
                margin: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
                child: ListTile(
                  leading: const CircleAvatar(
                    child: Icon(Icons.local_shipping),
                  ),
                  title: Text(
                    item['indirizzo_destinazione'] ?? item['indirizzo'] ?? 'Indirizzo N/D',
                    style: const TextStyle(fontWeight: FontWeight.bold),
                  ),
                  subtitle: Text('ID: ${item['id']} - Stato: $stato'),
                  trailing: const Icon(Icons.arrow_forward_ios, size: 16),
                  onTap: () async {
                    await Navigator.push(
                      context,
                      MaterialPageRoute(
                        builder: (context) => DettaglioSpedizioneScreen(spedizione: item),
                      ),
                    );
                    _ricaricaSpedizioni();
                  },
                ),
              );
            },
          );
        },
      ),
    );
  }
}